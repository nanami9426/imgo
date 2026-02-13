package middlewares

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/internal/utils"
)

var (
	// 通过函数变量注入，方便单元测试替换依赖而不需要真实 Redis。
	getRateLimitConfigFn         = utils.GetRateLimitConfig
	consumeChatCompletionQuotaFn = utils.ConsumeChatCompletionQuota
)

// RateLimitMiddleware 仅拦截 POST /v1/chat/completions 做双维度限流。
//
// 执行流程：
// 1) 非目标路由直接放行；
// 2) 从上下文读取 user_id（依赖 AuthMiddleware）；
// 3) 读取限流配置，若两个维度都关闭则放行；
// 4) 计算请求级和 token 级成本；
// 5) 调用 Redis 原子脚本检查+扣减；
// 6) 超限返回 429，Redis 异常按 fail-open 放行。
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 仅对 chat/completions 生效，避免影响其他 /v1 路由。
		if c.Request.Method != http.MethodPost || !shouldLogAPIPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// 鉴权中间件应该已写入 user_id，这里做防御性校验。
		userID, ok := parseUserIDFromContext(c)
		if !ok || userID <= 0 {
			utils.Abort(c, http.StatusUnauthorized, utils.StatUnauthorized, "token无效或已过期", nil)
			return
		}

		cfg := getRateLimitConfigFn()
		// 默认关闭限流：两个维度配额都 <=0 时，不做任何限流处理。
		if cfg.RequestPerMin <= 0 && cfg.TokenPerMin <= 0 {
			c.Next()
			return
		}

		// 请求级固定每次消耗 1。
		reqCost := int64(1)
		tokenCost := int64(0)
		if cfg.TokenPerMin > 0 {
			// token 级开启时才读取并解析 body，避免无意义的开销。
			rawBody, err := io.ReadAll(c.Request.Body)
			if err != nil {
				utils.Abort(c, http.StatusBadRequest, utils.StatInvalidParam, "读取请求体失败", err)
				return
			}
			// body 被读取后需要恢复，保证后续 ChatHistoryMiddleware/上游转发可继续读取。
			restoreRequestBody(c, rawBody)

			payload := map[string]interface{}{}
			dec := json.NewDecoder(bytes.NewReader(rawBody))
			dec.UseNumber()
			if err := dec.Decode(&payload); err != nil {
				utils.Abort(c, http.StatusBadRequest, utils.StatInvalidParam, "请求体必须是合法JSON对象", err)
				return
			}

			// prompt_tokens_est 仅按本次请求 messages 估算，不包含历史拼接。
			promptTokensEst := estimatePromptTokens(payload)
			maxTokens := parseMaxTokens(payload, cfg.DefaultMaxTokens)
			tokenCost = utils.CalculateTokenCost(promptTokensEst, maxTokens, cfg.TokenK)
			// 只要 token 级开启，单次请求至少消耗 1，避免“零成本请求”。
			if tokenCost < 1 {
				tokenCost = 1
			}
		}

		if err := consumeChatCompletionQuotaFn(c.Request.Context(), userID, reqCost, tokenCost); err != nil {
			var limitErr *utils.RateLimitExceededError
			if errors.As(err, &limitErr) {
				dimension := strings.TrimSpace(string(limitErr.Dimension))
				if dimension == "" {
					dimension = string(utils.RateLimitDimensionRequest)
				}
				// 维度信息放在 details，便于前端/调用方区分 request 或 token 触发。
				utils.Abort(
					c,
					http.StatusTooManyRequests,
					utils.StatTooManyRequests,
					"请求过于频繁，请稍后重试",
					errors.New("dimension="+dimension),
				)
				return
			}
			// 非超限错误（如 Redis 抖动）按“失败放行”处理，优先保证服务可用性。
			utils.Log.Errorf("rate limit check failed (fail-open): user_id=%d err=%v", userID, err)
		}

		c.Next()
	}
}

// restoreRequestBody 把已读的 body 重新挂回请求，避免后续中间件/handler拿到空 body。
func restoreRequestBody(c *gin.Context, body []byte) {
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	c.Request.Header.Set("Content-Length", strconv.Itoa(len(body)))
}

// estimatePromptTokens 估算 prompt token：
// 1) 遍历 messages；
// 2) 累加 role/content 的 UTF-8 字节数；
// 3) 按 ceil(bytes/4) 转成 token 估算值。
// 说明：这是低成本估算，不是 tokenizer 的精确值。
func estimatePromptTokens(payload map[string]interface{}) int64 {
	rawMessages, ok := payload["messages"]
	if !ok {
		return 0
	}
	messages, ok := rawMessages.([]interface{})
	if !ok || len(messages) == 0 {
		return 0
	}

	totalBytes := int64(0)
	for _, item := range messages {
		msg, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if role, ok := msg["role"].(string); ok {
			totalBytes += int64(len([]byte(role)))
		}
		totalBytes += estimateContentBytes(msg["content"])
	}
	if totalBytes <= 0 {
		return 0
	}
	return (totalBytes + 3) / 4
}

// estimateContentBytes 统计 content 的文本字节数。
// 兼容三类输入：
// 1) 纯字符串 content；
// 2) 数组结构（例如多段内容）；
// 3) 对象结构中的 text/content 字段。
func estimateContentBytes(v interface{}) int64 {
	switch val := v.(type) {
	case string:
		return int64(len([]byte(val)))
	case []interface{}:
		total := int64(0)
		for _, item := range val {
			total += estimateContentBytes(item)
		}
		return total
	case map[string]interface{}:
		if text, ok := val["text"].(string); ok {
			return int64(len([]byte(text)))
		}
		if content, ok := val["content"].(string); ok {
			return int64(len([]byte(content)))
		}
		return 0
	default:
		return 0
	}
}

// parseMaxTokens 解析 max_tokens；缺失或非法时回退到配置默认值。
func parseMaxTokens(payload map[string]interface{}, fallback int64) int64 {
	if fallback <= 0 {
		fallback = 256
	}
	raw, ok := payload["max_tokens"]
	if !ok {
		return fallback
	}

	n, ok := parseInt64Value(raw)
	if !ok || n < 0 {
		return fallback
	}
	return n
}

// parseInt64Value 兼容解析常见 JSON 类型为 int64，供 max_tokens 等字段复用。
func parseInt64Value(v interface{}) (int64, bool) {
	switch val := v.(type) {
	case int:
		return int64(val), true
	case int64:
		return val, true
	case uint:
		return int64(val), true
	case uint64:
		return int64(val), true
	case float64:
		return int64(val), true
	case json.Number:
		if n, err := val.Int64(); err == nil {
			return n, true
		}
		if f, err := val.Float64(); err == nil {
			return int64(f), true
		}
		return 0, false
	case string:
		s := strings.TrimSpace(val)
		if s == "" {
			return 0, false
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n, true
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f), true
		}
		return 0, false
	default:
		return 0, false
	}
}
