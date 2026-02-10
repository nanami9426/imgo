package router

import (
	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/internal/service"
)

func RegisterUsageRoutes(r *gin.Engine) {
	usage := r.Group("/usage")
	{
		usage.POST("/stats", service.GetUsageStats)
		usage.POST("/total", service.GetTotalUsage)
	}

}
