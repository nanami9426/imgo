package models

import (
	"time"

	"github.com/nanami9426/imgo/internal/utils"
)

type APIUsage struct {
	UsageID       int64     `gorm:"primarykey"`
	UserID        int64     `gorm:"index"` // 用户ID
	Endpoint      string    // 调用的端点（如 /v1/chat/completions）
	Model         string    // 使用的模型名称
	RequestMethod string    // HTTP 方法
	StatusCode    int       // 响应状态码
	InputTokens   int       // 输入 Token 数（如果有）
	OutputTokens  int       // 输出 Token 数（如果有）
	TotalTokens   int       // 总 Token 数
	LatencyMs     int       // 请求延迟（毫秒）
	RequestSize   int       // 请求体大小（字节）
	ResponseSize  int       // 响应体大小（字节）
	ErrorMsg      string    // 错误信息（成功为空）
	CreatedAt     time.Time `gorm:"index"`
	Basic
}

func (a *APIUsage) TableName() string {
	return "api_usage"
}

// 记录 API 调用
func CreateAPIUsage(usage *APIUsage) error {
	return utils.DB.Create(usage).Error
}

// 查询用户的用量统计（按天）
func GetUserDailyUsage(userID int64, date string) ([]*APIUsage, error) {
	var usages []*APIUsage
	result := utils.DB.Where("user_id = ? AND DATE(created_at) = ?", userID, date).Find(&usages)
	return usages, result.Error
}

// 查询用户的总用量统计
func GetUserTotalUsage(userID int64) (map[string]interface{}, error) {
	type UsageStat struct {
		TotalRequests int64
		SuccessCount  int64
		FailCount     int64
		TotalTokens   int64
		AvgLatencyMs  float64
	}

	var stat UsageStat
	err := utils.DB.
		Model(&APIUsage{}).
		Where("user_id = ?", userID).
		Select(
			"COUNT(*) as total_requests",
			"SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as success_count",
			"SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) as fail_count",
			"SUM(total_tokens) as total_tokens",
			"AVG(latency_ms) as avg_latency_ms",
		).
		Scan(&stat).Error

	return map[string]interface{}{
		"total_requests": stat.TotalRequests,
		"success_count":  stat.SuccessCount,
		"fail_count":     stat.FailCount,
		"total_tokens":   stat.TotalTokens,
		"avg_latency_ms": stat.AvgLatencyMs,
	}, err
}
