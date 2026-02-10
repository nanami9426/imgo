package middlewares

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/internal/models"
	"github.com/nanami9426/imgo/internal/utils"
)

const (
	contextKeyChatCompletionResponseBody = "chat_completion_response_body"
	responseHeaderConversationID         = "X-Conversation-ID"

	defaultHistoryMaxMessages = 20
	maxHistoryMaxMessages     = 200
	maxConversationTitleRunes = 30
)

type chatMessagePayload struct {
	Raw     map[string]interface{}
	Role    string
	Content string
}

// ChatHistoryMiddleware 为 /v1/chat/completions 增加会话能力：
// 1) 解析并消费 conversation_id/new_chat
// 2) 续聊时自动拼接历史消息
// 3) 预写入 user/system 消息，响应后补写 assistant 消息
func ChatHistoryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost || !shouldLogAPIPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		userID, ok := parseUserIDFromContext(c)
		if !ok || userID <= 0 {
			utils.Abort(c, http.StatusUnauthorized, utils.StatUnauthorized, "token无效或已过期", nil)
			return
		}

		rawBody, err := io.ReadAll(c.Request.Body)
		if err != nil {
			utils.Abort(c, http.StatusBadRequest, utils.StatInvalidParam, "读取请求体失败", err)
			return
		}

		payload := map[string]interface{}{}
		dec := json.NewDecoder(bytes.NewReader(rawBody))
		dec.UseNumber()
		if err := dec.Decode(&payload); err != nil {
			utils.Abort(c, http.StatusBadRequest, utils.StatInvalidParam, "请求体必须是合法JSON对象", err)
			return
		}

		// 当前请求里 model/messages 是会话持久化和上下文拼接的基础输入。
		modelName, _ := payload["model"].(string)
		currentMessages, err := parseRequestMessages(payload)
		if err != nil {
			utils.Abort(c, http.StatusBadRequest, utils.StatInvalidParam, err.Error(), nil)
			return
		}

		conversationID, hasConversationID, newChat, err := consumeConversationOptions(payload)
		if err != nil {
			utils.Abort(c, http.StatusBadRequest, utils.StatInvalidParam, err.Error(), nil)
			return
		}

		// 有 conversation_id 且没有 new_chat 时认为是续聊。
		isContinue := hasConversationID && !newChat
		if isContinue {
			belongs, err := models.ConversationBelongsToUser(conversationID, userID)
			if err != nil {
				utils.Abort(c, http.StatusInternalServerError, utils.StatDatabaseError, "查询会话失败", err)
				return
			}
			if !belongs {
				utils.Abort(c, http.StatusNotFound, utils.StatNotFound, "会话不存在", nil)
				return
			}
			if err := validateContinueMessages(currentMessages); err != nil {
				utils.Abort(c, http.StatusBadRequest, utils.StatInvalidParam, err.Error(), nil)
				return
			}
		} else {
			// 新会话在第一轮请求前创建，方便后续消息统一挂到 conversation_id。
			conversationID = utils.GenerateID()
			conversation := &models.LLMConversation{
				ConversationID: conversationID,
				UserID:         userID,
				Title:          buildConversationTitle(currentMessages),
				Model:          strings.TrimSpace(modelName),
				LastMessageAt:  time.Now().UTC(),
			}
			if err := models.CreateLLMConversation(conversation); err != nil {
				utils.Abort(c, http.StatusInternalServerError, utils.StatDatabaseError, "创建会话失败", err)
				return
			}
		}

		// 续聊时把数据库历史和本轮输入合并，最终写回给上游模型的 messages。
		mergedMessages, err := buildUpstreamMessages(conversationID, isContinue, currentMessages)
		if err != nil {
			utils.Abort(c, http.StatusInternalServerError, utils.StatDatabaseError, "组装历史消息失败", err)
			return
		}
		payload["messages"] = mergedMessages

		rewrittenBody, err := json.Marshal(payload)
		if err != nil {
			utils.Abort(c, http.StatusInternalServerError, utils.StatInternalError, "重写请求失败", err)
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(rewrittenBody))
		c.Request.ContentLength = int64(len(rewrittenBody))
		c.Request.Header.Set("Content-Length", strconv.Itoa(len(rewrittenBody)))

		// 通过响应头把会话ID返回前端，便于后续继续聊天。
		c.Set("conversation_id", conversationID)
		c.Writer.Header().Set(responseHeaderConversationID, strconv.FormatInt(conversationID, 10))

		// 在转发前先写入本轮 user/system 消息；assistant 需要等待上游响应后再落库。
		userMessages, err := buildMessagesForStorage(userID, conversationID, modelName, currentMessages)
		if err != nil {
			utils.Abort(c, http.StatusBadRequest, utils.StatInvalidParam, err.Error(), nil)
			return
		}
		if err := models.CreateLLMConversationMessages(userMessages); err != nil {
			utils.Abort(c, http.StatusInternalServerError, utils.StatDatabaseError, "保存会话消息失败", err)
			return
		}
		if err := models.RefreshLLMConversationStats(conversationID, modelName); err != nil {
			utils.Abort(c, http.StatusInternalServerError, utils.StatDatabaseError, "更新会话统计失败", err)
			return
		}

		c.Next()

		// APILoggingMiddleware 会把响应体放到 context，这里读取后解析 assistant 内容并落库。
		responseModel := strings.TrimSpace(modelName)
		if responseBody, ok := c.Get(contextKeyChatCompletionResponseBody); ok {
			if body, ok := responseBody.([]byte); ok && len(body) > 0 {
				content, parsedModel := extractAssistantContentAndModel(body)
				if strings.TrimSpace(parsedModel) != "" {
					responseModel = strings.TrimSpace(parsedModel)
				}
				if c.Writer.Status() >= 200 && c.Writer.Status() < 300 && strings.TrimSpace(content) != "" {
					if err := saveAssistantMessage(userID, conversationID, responseModel, content); err != nil {
						utils.Log.Errorf("failed to save assistant message: %v", err)
						return
					}
				}
			}
		}
		if err := models.RefreshLLMConversationStats(conversationID, responseModel); err != nil {
			utils.Log.Errorf("failed to refresh conversation stats: %v", err)
		}
	}
}

// parseUserIDFromContext 把鉴权中间件写入的 user_id 统一转成 int64。
func parseUserIDFromContext(c *gin.Context) (int64, bool) {
	v, ok := c.Get("user_id")
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case int64:
		return val, true
	case int:
		return int64(val), true
	case uint:
		return int64(val), true
	case uint64:
		return int64(val), true
	case float64:
		return int64(val), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

// parseRequestMessages 只接受文本 content，避免把复杂多模态结构直接落库导致脏数据。
func parseRequestMessages(payload map[string]interface{}) ([]chatMessagePayload, error) {
	rawMessages, ok := payload["messages"]
	if !ok {
		return nil, errors.New("messages 不能为空")
	}
	list, ok := rawMessages.([]interface{})
	if !ok || len(list) == 0 {
		return nil, errors.New("messages 必须是非空数组")
	}

	messages := make([]chatMessagePayload, 0, len(list))
	for _, item := range list {
		msg, ok := item.(map[string]interface{})
		if !ok {
			return nil, errors.New("messages 中元素必须是对象")
		}
		roleRaw, ok := msg["role"].(string)
		if !ok || strings.TrimSpace(roleRaw) == "" {
			return nil, errors.New("messages.role 必须是非空字符串")
		}
		content, ok := msg["content"].(string)
		if !ok {
			return nil, errors.New("messages.content 仅支持文本字符串")
		}
		role := strings.ToLower(strings.TrimSpace(roleRaw))
		msg["role"] = role
		msg["content"] = content
		messages = append(messages, chatMessagePayload{
			Raw:     msg,
			Role:    role,
			Content: content,
		})
	}
	return messages, nil
}

// consumeConversationOptions 从请求体中读取网关扩展字段，并将其从 payload 删除。
// 删除的原因：上游 vLLM 接口不识别这些字段。
func consumeConversationOptions(payload map[string]interface{}) (conversationID int64, hasConversationID bool, newChat bool, err error) {
	if rawNewChat, ok := payload["new_chat"]; ok {
		newChat, err = parseBool(rawNewChat)
		if err != nil {
			return 0, false, false, errors.New("new_chat 必须是布尔值")
		}
		delete(payload, "new_chat")
	}
	if rawConversationID, ok := payload["conversation_id"]; ok {
		conversationID, err = parseInt64(rawConversationID)
		if err != nil || conversationID <= 0 {
			return 0, false, false, errors.New("conversation_id 必须是正整数")
		}
		hasConversationID = true
		delete(payload, "conversation_id")
	}
	return conversationID, hasConversationID, newChat, nil
}

func parseBool(v interface{}) (bool, error) {
	switch val := v.(type) {
	case bool:
		return val, nil
	case string:
		return strconv.ParseBool(strings.TrimSpace(val))
	default:
		return false, errors.New("invalid bool")
	}
}

func parseInt64(v interface{}) (int64, error) {
	switch val := v.(type) {
	case int:
		return int64(val), nil
	case int64:
		return val, nil
	case float64:
		return int64(val), nil
	case json.Number:
		return val.Int64()
	case string:
		return strconv.ParseInt(strings.TrimSpace(val), 10, 64)
	default:
		return 0, errors.New("invalid int")
	}
}

// 续聊严格校验：只允许 user 或 system+user，防止前端重复传历史导致上下文膨胀。
func validateContinueMessages(messages []chatMessagePayload) error {
	if len(messages) == 0 {
		return errors.New("续聊时 messages 不能为空")
	}
	if len(messages) > 2 {
		return errors.New("续聊时只允许 1 条 user，或 1 条 system + 1 条 user")
	}
	if len(messages) == 1 {
		if messages[0].Role != "user" {
			return errors.New("续聊时单条 messages 必须为 user")
		}
		return nil
	}
	if messages[0].Role != "system" || messages[1].Role != "user" {
		return errors.New("续聊时两条 messages 必须按 system, user 顺序传入")
	}
	return nil
}

// buildUpstreamMessages 根据是否续聊决定是否注入历史上下文。
func buildUpstreamMessages(conversationID int64, isContinue bool, currentMessages []chatMessagePayload) ([]map[string]interface{}, error) {
	if !isContinue {
		upstream := make([]map[string]interface{}, 0, len(currentMessages))
		for _, msg := range currentMessages {
			upstream = append(upstream, msg.Raw)
		}
		return upstream, nil
	}
	history, err := models.GetRecentLLMConversationMessages(conversationID, historyMessageLimit())
	if err != nil {
		return nil, err
	}
	return mergeHistoryAndCurrentMessages(history, currentMessages)
}

// historyMessageLimit 从配置读取历史条数上限，未配置时使用默认值。
func historyMessageLimit() int {
	n := utils.V.GetInt("vllm.history_max_messages")
	if n <= 0 {
		n = defaultHistoryMaxMessages
	}
	if n > maxHistoryMaxMessages {
		n = maxHistoryMaxMessages
	}
	return n
}

// mergeHistoryAndCurrentMessages 保证顺序是：历史 -> 本轮输入。
func mergeHistoryAndCurrentMessages(history []*models.LLMConversationMessage, currentMessages []chatMessagePayload) ([]map[string]interface{}, error) {
	merged := make([]map[string]interface{}, 0, len(history)+len(currentMessages))
	for _, item := range history {
		msg := map[string]interface{}{}
		if strings.TrimSpace(item.MessageJSON) != "" {
			if err := json.Unmarshal([]byte(item.MessageJSON), &msg); err == nil {
				merged = append(merged, msg)
				continue
			}
		}
		merged = append(merged, map[string]interface{}{
			"role":    item.Role,
			"content": item.Content,
		})
	}
	for _, item := range currentMessages {
		merged = append(merged, item.Raw)
	}
	return merged, nil
}

// buildMessagesForStorage 只落 user/system；assistant 在响应返回后再单独写入。
func buildMessagesForStorage(userID int64, conversationID int64, model string, messages []chatMessagePayload) ([]*models.LLMConversationMessage, error) {
	out := make([]*models.LLMConversationMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Role != "system" && msg.Role != "user" {
			continue
		}
		raw, err := json.Marshal(msg.Raw)
		if err != nil {
			return nil, err
		}
		out = append(out, &models.LLMConversationMessage{
			MessageID:      utils.GenerateID(),
			ConversationID: conversationID,
			UserID:         userID,
			Role:           msg.Role,
			Content:        msg.Content,
			MessageJSON:    string(raw),
			Model:          strings.TrimSpace(model),
		})
	}
	return out, nil
}

// 新会话标题默认取首条 user 消息前 N 个字符。
func buildConversationTitle(messages []chatMessagePayload) string {
	for _, msg := range messages {
		if msg.Role == "user" {
			title := strings.TrimSpace(msg.Content)
			if title != "" {
				return truncateRunes(title, maxConversationTitleRunes)
			}
		}
	}
	return "新对话"
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= limit {
		return s
	}
	return string(rs[:limit])
}

// saveAssistantMessage 在拿到上游响应后补写 assistant。
func saveAssistantMessage(userID int64, conversationID int64, model string, content string) error {
	raw, err := json.Marshal(map[string]interface{}{
		"role":    "assistant",
		"content": content,
	})
	if err != nil {
		return err
	}
	msg := &models.LLMConversationMessage{
		MessageID:      utils.GenerateID(),
		ConversationID: conversationID,
		UserID:         userID,
		Role:           "assistant",
		Content:        content,
		MessageJSON:    string(raw),
		Model:          strings.TrimSpace(model),
	}
	return models.CreateLLMConversationMessages([]*models.LLMConversationMessage{msg})
}

// extractAssistantContentAndModel 同时兼容 JSON 和 SSE 两种响应格式。
func extractAssistantContentAndModel(body []byte) (string, string) {
	if content, model, ok := parseAssistantFromJSON(body); ok {
		return content, model
	}
	return parseAssistantFromSSE(body)
}

type responseChoice struct {
	Message *responseMessage `json:"message"`
	Delta   *responseMessage `json:"delta"`
}

type responseMessage struct {
	Content interface{} `json:"content"`
}

type responsePayload struct {
	Model   string           `json:"model"`
	Choices []responseChoice `json:"choices"`
}

func parseAssistantFromJSON(body []byte) (string, string, bool) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var resp responsePayload
	if err := dec.Decode(&resp); err != nil {
		return "", "", false
	}
	var builder strings.Builder
	for _, choice := range resp.Choices {
		if choice.Message != nil {
			if text := contentToString(choice.Message.Content); text != "" {
				builder.WriteString(text)
			}
		}
		if choice.Delta != nil {
			if text := contentToString(choice.Delta.Content); text != "" {
				builder.WriteString(text)
			}
		}
	}
	return builder.String(), strings.TrimSpace(resp.Model), true
}

// parseAssistantFromSSE 会拼接多个 delta chunk，得到完整 assistant 文本。
func parseAssistantFromSSE(body []byte) (string, string) {
	lines := bytes.Split(body, []byte("\n"))
	var dataBuf []byte
	var model string
	var builder strings.Builder
	flush := func() {
		if len(dataBuf) == 0 {
			return
		}
		data := bytes.TrimSpace(dataBuf)
		dataBuf = nil
		if len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) {
			return
		}
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		var chunk responsePayload
		if err := dec.Decode(&chunk); err != nil {
			return
		}
		if model == "" && strings.TrimSpace(chunk.Model) != "" {
			model = strings.TrimSpace(chunk.Model)
		}
		for _, choice := range chunk.Choices {
			if choice.Delta != nil {
				if text := contentToString(choice.Delta.Content); text != "" {
					builder.WriteString(text)
				}
			}
			if choice.Message != nil {
				if text := contentToString(choice.Message.Content); text != "" {
					builder.WriteString(text)
				}
			}
		}
	}

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			flush()
			continue
		}
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(data) == 0 {
			continue
		}
		if len(dataBuf) > 0 {
			dataBuf = append(dataBuf, '\n')
		}
		dataBuf = append(dataBuf, data...)
	}
	flush()
	return builder.String(), model
}

func contentToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	default:
		return ""
	}
}
