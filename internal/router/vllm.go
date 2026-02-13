package router

import (
	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/internal/router/middlewares"
	"github.com/nanami9426/imgo/internal/service"
)

func RigisterVLLMRoutes(r *gin.Engine) {
	v1 := r.Group("/v1")
	v1.Use(middlewares.AuthMiddleware())
	v1.Use(middlewares.RateLimitMiddleware())
	// 先做会话处理（改写请求、写入历史），再做 API 用量统计。
	v1.Use(middlewares.ChatHistoryMiddleware())
	v1.Use(middlewares.APILoggingMiddleware())
	v1.GET("/conversations", service.GetConversations)
	v1.GET("/conversations/:conversation_id/messages", service.GetConversationMessages)
	v1.POST("/chat/completions", service.ChatCompletionsHandler())
	v1.Any("/:path", service.ProxyToVLLM())
	v1.Any("/:path/*any", service.ProxyToVLLM())
}
