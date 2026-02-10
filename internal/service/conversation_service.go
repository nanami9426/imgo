package service

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/internal/models"
	"github.com/nanami9426/imgo/internal/utils"
	"gorm.io/gorm"
)

const (
	defaultConversationPage     = 1
	defaultConversationPageSize = 20
	maxConversationPageSize     = 100

	defaultMessagePage     = 1
	defaultMessagePageSize = 50
	maxMessagePageSize     = 200
)

type conversationResp struct {
	ConversationID     int64  `json:"conversation_id"`
	Title              string `json:"title"`
	Model              string `json:"model"`
	MessageCount       int    `json:"message_count"`
	LastMessagePreview string `json:"last_message_preview"`
	LastMessageAt      string `json:"last_message_at"`
}

type conversationMessageResp struct {
	MessageID  int64  `json:"message_id"`
	Role       string `json:"role"`
	Content    string `json:"content"`
	Model      string `json:"model"`
	CreatedAt  string `json:"created_at"`
	ModifiedAt string `json:"updated_at"`
}

// GetConversations 返回当前登录用户的会话列表（按最近消息时间倒序）。
func GetConversations(c *gin.Context) {
	userID, ok := parseUserID(c)
	if !ok || userID <= 0 {
		utils.Fail(c, http.StatusUnauthorized, utils.StatUnauthorized, "token无效或已过期", nil)
		return
	}
	page, pageSize, err := parsePagination(c, defaultConversationPage, defaultConversationPageSize, maxConversationPageSize)
	if err != nil {
		utils.Fail(c, http.StatusOK, utils.StatInvalidParam, err.Error(), nil)
		return
	}

	total, err := models.CountLLMConversationsByUser(userID)
	if err != nil {
		utils.Fail(c, http.StatusOK, utils.StatDatabaseError, "查询会话列表失败", err)
		return
	}

	// 列表接口使用 page/page_size，内部转换成 offset/limit。
	offset := (page - 1) * pageSize
	conversations, err := models.ListLLMConversationsByUser(userID, offset, pageSize)
	if err != nil {
		utils.Fail(c, http.StatusOK, utils.StatDatabaseError, "查询会话列表失败", err)
		return
	}

	items := make([]conversationResp, 0, len(conversations))
	for _, item := range conversations {
		items = append(items, conversationResp{
			ConversationID:     item.ConversationID,
			Title:              item.Title,
			Model:              item.Model,
			MessageCount:       item.MessageCount,
			LastMessagePreview: item.LastMessagePreview,
			LastMessageAt:      item.LastMessageAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		})
	}

	utils.Success(c, gin.H{
		"list":      items,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

// GetConversationMessages 返回指定会话的消息列表，并校验会话归属。
func GetConversationMessages(c *gin.Context) {
	userID, ok := parseUserID(c)
	if !ok || userID <= 0 {
		utils.Fail(c, http.StatusUnauthorized, utils.StatUnauthorized, "token无效或已过期", nil)
		return
	}
	conversationID, err := strconv.ParseInt(strings.TrimSpace(c.Param("conversation_id")), 10, 64)
	if err != nil || conversationID <= 0 {
		utils.Fail(c, http.StatusOK, utils.StatInvalidParam, "conversation_id 必须是正整数", nil)
		return
	}
	page, pageSize, err := parsePagination(c, defaultMessagePage, defaultMessagePageSize, maxMessagePageSize)
	if err != nil {
		utils.Fail(c, http.StatusOK, utils.StatInvalidParam, err.Error(), nil)
		return
	}

	conversation, err := models.GetLLMConversationByIDAndUser(conversationID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Fail(c, http.StatusOK, utils.StatNotFound, "会话不存在", nil)
			return
		}
		utils.Fail(c, http.StatusOK, utils.StatDatabaseError, "查询会话失败", err)
		return
	}

	total, err := models.CountLLMConversationMessages(conversationID)
	if err != nil {
		utils.Fail(c, http.StatusOK, utils.StatDatabaseError, "查询会话消息失败", err)
		return
	}

	// 消息详情同样走分页，避免长会话一次性返回过大。
	offset := (page - 1) * pageSize
	messages, err := models.ListLLMConversationMessages(conversationID, offset, pageSize)
	if err != nil {
		utils.Fail(c, http.StatusOK, utils.StatDatabaseError, "查询会话消息失败", err)
		return
	}

	items := make([]conversationMessageResp, 0, len(messages))
	for _, msg := range messages {
		items = append(items, conversationMessageResp{
			MessageID:  msg.MessageID,
			Role:       msg.Role,
			Content:    msg.Content,
			Model:      msg.Model,
			CreatedAt:  msg.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
			ModifiedAt: msg.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		})
	}

	utils.Success(c, gin.H{
		"conversation": conversationResp{
			ConversationID:     conversation.ConversationID,
			Title:              conversation.Title,
			Model:              conversation.Model,
			MessageCount:       conversation.MessageCount,
			LastMessagePreview: conversation.LastMessagePreview,
			LastMessageAt:      conversation.LastMessageAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		},
		"messages":  items,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

// parseUserID 统一处理鉴权中间件写入 user_id 的多种类型。
func parseUserID(c *gin.Context) (int64, bool) {
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

// parsePagination 统一处理分页默认值、边界和参数合法性。
func parsePagination(c *gin.Context, defaultPage int, defaultPageSize int, maxPageSize int) (int, int, error) {
	page := defaultPage
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return 0, 0, errors.New("page 必须是正整数")
		}
		page = v
	}
	pageSize := defaultPageSize
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return 0, 0, errors.New("page_size 必须是正整数")
		}
		pageSize = v
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize, nil
}
