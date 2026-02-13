package utils

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// 限流默认值说明：
// 1) request_per_min/token_per_min 默认 0，表示“默认关闭”，由业务显式开启。
// 2) token_k 与 default_max_tokens 是 token 级成本估算用参数。
// 3) window_seconds 使用固定窗口，默认 60 秒。
// 4) redis_prefix 用于隔离不同业务的 key 命名空间。
const (
	defaultRateLimitRequestPerMin   int64  = 0
	defaultRateLimitTokenPerMin     int64  = 0
	defaultRateLimitTokenK          int64  = 100
	defaultRateLimitDefaultMaxToken int64  = 256
	defaultRateLimitWindowSeconds   int64  = 60
	defaultRateLimitRedisPrefix     string = "rl:chat"
)

// config/app.yaml 对应的配置键。
const (
	cfgRateLimitRequestPerMin   = "rate_limit.request_per_min"
	cfgRateLimitTokenPerMin     = "rate_limit.token_per_min"
	cfgRateLimitTokenK          = "rate_limit.token_k"
	cfgRateLimitDefaultMaxToken = "rate_limit.default_max_tokens"
	cfgRateLimitWindowSeconds   = "rate_limit.window_seconds"
	cfgRateLimitRedisPrefix     = "rate_limit.redis_prefix"
)

// RateLimitDimension 标识哪个维度触发了限流，方便上层返回更明确的错误信息。
type RateLimitDimension string

const (
	RateLimitDimensionRequest RateLimitDimension = "request"
	RateLimitDimensionToken   RateLimitDimension = "token"
)

// RateLimitConfig 为 chat/completions 的限流配置。
// RequestPerMin:
//
//	请求级额度，每次请求固定消耗 1。
//
// TokenPerMin:
//
//	token 级额度，消耗值由公式计算。
//
// TokenK:
//
//	token 级成本缩放参数，cost = ceil((prompt_tokens_est + max_tokens)/K)。
//
// DefaultMaxTokens:
//
//	请求没带 max_tokens 时的兜底值，避免绕过 token 限流。
//
// WindowSeconds:
//
//	固定窗口长度（秒）。
//
// RedisPrefix:
//
//	Redis key 前缀，避免与其他业务冲突。
type RateLimitConfig struct {
	RequestPerMin    int64
	TokenPerMin      int64
	TokenK           int64
	DefaultMaxTokens int64
	WindowSeconds    int64
	RedisPrefix      string
}

// RateLimitExceededError 用于向上层传递“已超限 + 维度信息”。
type RateLimitExceededError struct {
	Dimension RateLimitDimension
}

func (e *RateLimitExceededError) Error() string {
	d := strings.TrimSpace(string(e.Dimension))
	if d == "" {
		return "rate limit exceeded"
	}
	return "rate limit exceeded: " + d
}

var (
	// rateLimitConfig 是进程内缓存，避免每次请求都走 Viper 解析。
	// 配置仍可通过 InitRateLimitConfig 主动刷新。
	rateLimitConfig   RateLimitConfig
	rateLimitConfigMu sync.RWMutex
)

// consumeChatCompletionQuotaScript:
// 在 Redis 中原子执行“检查 + 扣减”，保证双维度 all-or-none：
// 1) 先检查 request/token 两个维度是否会超限；
// 2) 任何一个超限都直接返回，不做扣减；
// 3) 都未超限时再同时扣减，并为新窗口 key 设置 TTL。
//
// 参数约定：
// KEYS[1] = request key
// KEYS[2] = token key
// ARGV[1] = req_enabled (0/1)
// ARGV[2] = req_limit
// ARGV[3] = req_cost
// ARGV[4] = tok_enabled (0/1)
// ARGV[5] = tok_limit
// ARGV[6] = tok_cost
// ARGV[7] = ttl_seconds
//
// 返回：
// {1, ""}            => 通过
// {0, "request"}     => 请求级超限
// {0, "token"}       => token 级超限
var consumeChatCompletionQuotaScript = redis.NewScript(`
local req_enabled = tonumber(ARGV[1])
local req_limit = tonumber(ARGV[2])
local req_cost = tonumber(ARGV[3])
local tok_enabled = tonumber(ARGV[4])
local tok_limit = tonumber(ARGV[5])
local tok_cost = tonumber(ARGV[6])
local ttl = tonumber(ARGV[7])

if req_enabled == 1 then
	local req_current = tonumber(redis.call("GET", KEYS[1]) or "0")
	if req_current + req_cost > req_limit then
		return {0, "request"}
	end
end

if tok_enabled == 1 then
	local tok_current = tonumber(redis.call("GET", KEYS[2]) or "0")
	if tok_current + tok_cost > tok_limit then
		return {0, "token"}
	end
end

if req_enabled == 1 then
	local req_after = redis.call("INCRBY", KEYS[1], req_cost)
	if tonumber(req_after) == req_cost then
		redis.call("EXPIRE", KEYS[1], ttl)
	end
end

if tok_enabled == 1 then
	local tok_after = redis.call("INCRBY", KEYS[2], tok_cost)
	if tonumber(tok_after) == tok_cost then
		redis.call("EXPIRE", KEYS[2], ttl)
	end
end

return {1, ""}
`)

// InitRateLimitConfig 在服务启动阶段加载并缓存限流配置。
func InitRateLimitConfig() {
	setRateLimitConfig(loadRateLimitConfigFromViper())
}

// GetRateLimitConfig 返回当前可用配置。
// 优先返回缓存；缓存非法（例如未初始化）时会即时从 Viper 读取并修正。
func GetRateLimitConfig() RateLimitConfig {
	rateLimitConfigMu.RLock()
	cfg := rateLimitConfig
	rateLimitConfigMu.RUnlock()

	if cfg.WindowSeconds > 0 && cfg.TokenK > 0 && cfg.DefaultMaxTokens > 0 && strings.TrimSpace(cfg.RedisPrefix) != "" {
		return cfg
	}

	cfg = loadRateLimitConfigFromViper()
	setRateLimitConfig(cfg)
	return cfg
}

// setRateLimitConfig 以写锁更新配置缓存。
func setRateLimitConfig(cfg RateLimitConfig) {
	rateLimitConfigMu.Lock()
	rateLimitConfig = cfg
	rateLimitConfigMu.Unlock()
}

// loadRateLimitConfigFromViper 从 config/app.yaml 读取限流配置并做防御性修正。
// 修正规则：
// 1) 负值额度统一回退到默认值；
// 2) K、window、default_max_tokens <= 0 时回退默认值；
// 3) 空前缀回退默认前缀。
func loadRateLimitConfigFromViper() RateLimitConfig {
	cfg := RateLimitConfig{
		RequestPerMin:    V.GetInt64(cfgRateLimitRequestPerMin),
		TokenPerMin:      V.GetInt64(cfgRateLimitTokenPerMin),
		TokenK:           V.GetInt64(cfgRateLimitTokenK),
		DefaultMaxTokens: V.GetInt64(cfgRateLimitDefaultMaxToken),
		WindowSeconds:    V.GetInt64(cfgRateLimitWindowSeconds),
		RedisPrefix:      strings.TrimSpace(V.GetString(cfgRateLimitRedisPrefix)),
	}
	if cfg.RequestPerMin < 0 {
		cfg.RequestPerMin = defaultRateLimitRequestPerMin
	}
	if cfg.TokenPerMin < 0 {
		cfg.TokenPerMin = defaultRateLimitTokenPerMin
	}
	if cfg.TokenK <= 0 {
		cfg.TokenK = defaultRateLimitTokenK
	}
	if cfg.DefaultMaxTokens <= 0 {
		cfg.DefaultMaxTokens = defaultRateLimitDefaultMaxToken
	}
	if cfg.WindowSeconds <= 0 {
		cfg.WindowSeconds = defaultRateLimitWindowSeconds
	}
	if cfg.RedisPrefix == "" {
		cfg.RedisPrefix = defaultRateLimitRedisPrefix
	}
	return cfg
}

// CalculateTokenCost 计算 token 维度消耗：
// cost = ceil((prompt_tokens_est + max_tokens)/k)。
// 当总量 <= 0 时返回 0（上层可按业务决定是否最小收 1）。
func CalculateTokenCost(promptTokensEst int64, maxTokens int64, k int64) int64 {
	if k <= 0 {
		k = defaultRateLimitTokenK
	}
	total := promptTokensEst + maxTokens
	if total <= 0 {
		return 0
	}
	return (total + k - 1) / k
}

// BuildRateLimitWindowKeys 生成当前窗口的 request/token key。
// key 格式：
//
//	<prefix>:req:<user_id>:<window_id>
//	<prefix>:tok:<user_id>:<window_id>
//
// window_id = unix / window_seconds。
func BuildRateLimitWindowKeys(cfg RateLimitConfig, userID int64, now time.Time) (string, string, int64) {
	windowSeconds := cfg.WindowSeconds
	if windowSeconds <= 0 {
		windowSeconds = defaultRateLimitWindowSeconds
	}
	windowID := now.Unix() / windowSeconds
	reqKey := fmt.Sprintf("%s:req:%d:%d", cfg.RedisPrefix, userID, windowID)
	tokKey := fmt.Sprintf("%s:tok:%d:%d", cfg.RedisPrefix, userID, windowID)
	return reqKey, tokKey, windowID
}

// ConsumeChatCompletionQuota 执行一次 chat/completions 的双维度扣费检查。
// 行为要点：
// 1) request/token 两个维度都关闭时直接放行；
// 2) 使用 Lua 原子脚本保证并发场景不出现“只扣一半”；
// 3) 返回 nil 表示通过；
// 4) 返回 *RateLimitExceededError 表示超限（含维度）；
// 5) 其他 error 代表 Redis/解析异常，由上层决定 fail-open 或 fail-close。
func ConsumeChatCompletionQuota(ctx context.Context, userID int64, reqCost int64, tokenCost int64) error {
	cfg := GetRateLimitConfig()

	reqEnabled := cfg.RequestPerMin > 0 && reqCost > 0
	tokEnabled := cfg.TokenPerMin > 0 && tokenCost > 0
	if !reqEnabled && !tokEnabled {
		return nil
	}
	if RDB == nil {
		return errors.New("redis not initialized")
	}

	reqKey, tokKey, _ := BuildRateLimitWindowKeys(cfg, userID, time.Now().UTC())
	ttlSeconds := cfg.WindowSeconds * 2
	if ttlSeconds <= 0 {
		ttlSeconds = defaultRateLimitWindowSeconds * 2
	}

	res, err := consumeChatCompletionQuotaScript.Run(
		ctx,
		RDB,
		[]string{reqKey, tokKey},
		boolToInt(reqEnabled),
		cfg.RequestPerMin,
		reqCost,
		boolToInt(tokEnabled),
		cfg.TokenPerMin,
		tokenCost,
		ttlSeconds,
	).Result()
	if err != nil {
		return err
	}

	allowed, dimension, err := parseQuotaScriptResult(res)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	return &RateLimitExceededError{Dimension: RateLimitDimension(dimension)}
}

// boolToInt 把布尔值转换为 Lua 脚本可直接使用的 0/1。
func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// parseQuotaScriptResult 解析 Lua 返回值并进行类型校验。
func parseQuotaScriptResult(res interface{}) (bool, string, error) {
	parts, ok := res.([]interface{})
	if !ok || len(parts) < 2 {
		return false, "", errors.New("invalid rate limit script result")
	}

	allowed, ok := asInt64(parts[0])
	if !ok {
		return false, "", errors.New("invalid rate limit allowed flag")
	}
	dimension, _ := asString(parts[1])
	return allowed == 1, strings.TrimSpace(dimension), nil
}

// asInt64 用于解析 redis 脚本返回中的数字，兼容常见类型。
func asInt64(v interface{}) (int64, bool) {
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
		n, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	case []byte:
		n, err := strconv.ParseInt(strings.TrimSpace(string(val)), 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

// asString 用于解析 redis 脚本返回中的字符串，兼容 string/[]byte。
func asString(v interface{}) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case []byte:
		return string(val), true
	default:
		return "", false
	}
}
