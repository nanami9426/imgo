package service

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/internal/models"
	"github.com/nanami9426/imgo/internal/utils"
)

type UsageStatsReq struct {
	UserID int64  `json:"user_id" form:"user_id" binding:"required"`
	Date   string `json:"date" form:"date"` // YYYY-MM-DD 格式
}

type TotalUsageReq struct {
	UserID int64 `json:"user_id" form:"user_id" binding:"required"`
}

type UsageStatsResp struct {
	TotalRequests int64       `json:"total_requests"`
	SuccessCount  int64       `json:"success_count"`
	FailCount     int64       `json:"fail_count"`
	TotalTokens   int64       `json:"total_tokens"`
	AvgLatencyMs  float64     `json:"avg_latency_ms"`
	Details       []APIDetail `json:"details"`
}

type APIDetail struct {
	Endpoint     string  `json:"endpoint"`
	Count        int     `json:"count"`
	TotalTokens  int     `json:"total_tokens"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// @Summary 获取用户用量统计
// @Tags usage
// @Produce json
// @Router /usage/stats [post]
// @param user_id formData int64 true "用户ID"
// @param date formData string false "日期 (YYYY-MM-DD)"
func GetUsageStats(c *gin.Context) {
	var req UsageStatsReq
	if err := c.ShouldBind(&req); err != nil {
		utils.Fail(c, http.StatusOK, utils.StatInvalidParam, "参数错误", err)
		return
	}
	if req.UserID <= 0 {
		utils.Fail(c, http.StatusOK, utils.StatInvalidParam, "user_id 必须大于 0", nil)
		return
	}

	date := req.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	} else if _, err := time.Parse("2006-01-02", date); err != nil {
		utils.Fail(c, http.StatusOK, utils.StatInvalidParam, "date 格式错误，应为 YYYY-MM-DD", err)
		return
	}

	usages, err := models.GetUserDailyUsage(req.UserID, date)
	if err != nil {
		utils.Fail(c, http.StatusOK, utils.StatDatabaseError, "查询失败", err)
		return
	}

	// 统计数据
	totalTokens := int64(0)
	totalLatency := int64(0)
	successCount := int64(0)
	failCount := int64(0)

	endpointMap := make(map[string]*APIDetail)

	for _, usage := range usages {
		totalTokens += int64(usage.TotalTokens)
		totalLatency += int64(usage.LatencyMs)

		if usage.StatusCode >= 200 && usage.StatusCode < 300 {
			successCount++
		} else {
			failCount++
		}

		// 按端点分组
		if detail, ok := endpointMap[usage.Endpoint]; ok {
			detail.Count++
			detail.TotalTokens += usage.TotalTokens
			detail.AvgLatencyMs += float64(usage.LatencyMs)
		} else {
			endpointMap[usage.Endpoint] = &APIDetail{
				Endpoint:     usage.Endpoint,
				Count:        1,
				TotalTokens:  usage.TotalTokens,
				AvgLatencyMs: float64(usage.LatencyMs),
			}
		}
	}

	// 计算平均值
	avgLatency := 0.0
	if len(usages) > 0 {
		avgLatency = float64(totalLatency) / float64(len(usages))
	}

	// 计算每个端点的平均延迟
	for _, detail := range endpointMap {
		detail.AvgLatencyMs = detail.AvgLatencyMs / float64(detail.Count)
	}

	// 转换为数组
	var details []APIDetail
	for _, v := range endpointMap {
		details = append(details, *v)
	}

	resp := UsageStatsResp{
		TotalRequests: int64(len(usages)),
		SuccessCount:  successCount,
		FailCount:     failCount,
		TotalTokens:   totalTokens,
		AvgLatencyMs:  avgLatency,
		Details:       details,
	}

	utils.Success(c, resp)
}

// @Summary 获取用户总用量统计
// @Tags usage
// @Produce json
// @Router /usage/total [post]
// @param user_id formData int64 true "用户ID"
func GetTotalUsage(c *gin.Context) {
	req := &TotalUsageReq{}
	if err := c.ShouldBind(req); err != nil {
		utils.Fail(c, http.StatusOK, utils.StatInvalidParam, "参数错误", err)
		return
	}
	if req.UserID <= 0 {
		utils.Fail(c, http.StatusOK, utils.StatInvalidParam, "user_id 必须大于 0", nil)
		return
	}

	stats, err := models.GetUserTotalUsage(req.UserID)
	if err != nil {
		utils.Fail(c, http.StatusOK, utils.StatDatabaseError, "查询失败", err)
		return
	}

	utils.Success(c, stats)
}
