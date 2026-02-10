package middlewares

import (
	"bytes"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/internal/models"
	"github.com/nanami9426/imgo/internal/utils"
)

// 记录 API 调用的中间件
func APILoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// 记录请求体大小
		requestSize := c.Request.ContentLength
		if requestSize < 0 {
			requestSize = 0
		}

		// 拦截响应体
		writer := &responseWriter{ResponseWriter: c.Writer, body: []byte{}}
		c.Writer = writer

		// 继续处理请求
		c.Next()

		// 计算耗时
		latency := time.Since(startTime).Milliseconds()

		// 提取用户信息
		userID := int64(0)
		if v, ok := c.Get("user_id"); ok {
			switch val := v.(type) {
			case int64:
				userID = val
			case int:
				userID = int64(val)
			case uint:
				userID = int64(val)
			case uint64:
				userID = int64(val)
			case float64:
				userID = int64(val)
			case string:
				if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
					userID = parsed
				}
			}
		}

		// 创建使用记录
		responseSize := c.Writer.Size()
		if responseSize < 0 {
			responseSize = 0
		}

		usage := &models.APIUsage{
			UsageID:       utils.GenerateID(),
			UserID:        userID,
			Endpoint:      c.Request.URL.Path,
			RequestMethod: c.Request.Method,
			StatusCode:    c.Writer.Status(),
			RequestSize:   int(requestSize),
			ResponseSize:  int(responseSize),
			LatencyMs:     int(latency),
		}

		// 尝试从响应中提取 Token 信息
		extractTokenInfo(writer.body, usage)

		// 记录到数据库
		if err := models.CreateAPIUsage(usage); err != nil {
			utils.Log.Errorf("failed to create api usage record: %v", err)
		}
	}
}

type responseWriter struct {
	gin.ResponseWriter
	body []byte
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}

type openAIUsage struct {
	PromptTokens     *json.Number `json:"prompt_tokens"`
	CompletionTokens *json.Number `json:"completion_tokens"`
	TotalTokens      *json.Number `json:"total_tokens"`
}

type openAIError struct {
	Message string `json:"message"`
}

type openAIResp struct {
	Model string       `json:"model"`
	Usage *openAIUsage `json:"usage"`
	Error *openAIError `json:"error"`
}

func parseNumber(n *json.Number) (int, bool) {
	if n == nil {
		return 0, false
	}
	if i, err := n.Int64(); err == nil {
		return int(i), true
	}
	if f, err := n.Float64(); err == nil {
		return int(f), true
	}
	return 0, false
}

func applyModelAndError(resp *openAIResp, usage *models.APIUsage) {
	if resp == nil || usage == nil {
		return
	}
	if resp.Model != "" {
		usage.Model = resp.Model
	}
	if resp.Error != nil && resp.Error.Message != "" {
		usage.ErrorMsg = resp.Error.Message
	}
}

func applyUsage(resp *openAIResp, usage *models.APIUsage) bool {
	if resp == nil || usage == nil || resp.Usage == nil {
		return false
	}
	prompt, promptOK := parseNumber(resp.Usage.PromptTokens)
	completion, completionOK := parseNumber(resp.Usage.CompletionTokens)
	total, totalOK := parseNumber(resp.Usage.TotalTokens)

	if promptOK {
		usage.InputTokens = prompt
	}
	if completionOK {
		usage.OutputTokens = completion
	}
	if totalOK {
		usage.TotalTokens = total
	}
	if !totalOK && promptOK && completionOK {
		usage.TotalTokens = prompt + completion
		totalOK = true
	}
	return promptOK || completionOK || totalOK
}

func parseOpenAIJSON(body []byte) (*openAIResp, bool) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var resp openAIResp
	if err := dec.Decode(&resp); err != nil {
		return nil, false
	}
	return &resp, true
}

func parseOpenAISSE(body []byte) (*openAIResp, bool) {
	lines := bytes.Split(body, []byte("\n"))
	var last *openAIResp
	var lastWithUsage *openAIResp
	var dataBuf []byte
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
		var resp openAIResp
		if err := dec.Decode(&resp); err != nil {
			return
		}
		last = &resp
		if resp.Usage != nil {
			lastWithUsage = &resp
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
	if lastWithUsage != nil {
		return lastWithUsage, true
	}
	if last != nil {
		return last, true
	}
	return nil, false
}

// 从响应体中提取Token信息
func extractTokenInfo(body []byte, usage *models.APIUsage) {
	if usage == nil || len(body) == 0 {
		return
	}
	if resp, ok := parseOpenAIJSON(body); ok {
		applyModelAndError(resp, usage)
		if applyUsage(resp, usage) {
			return
		}
	}
	if resp, ok := parseOpenAISSE(body); ok {
		applyModelAndError(resp, usage)
		_ = applyUsage(resp, usage)
	}
}
