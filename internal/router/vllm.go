package router

import (
	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/internal/router/middlewares"
	"github.com/nanami9426/imgo/internal/service"
)

func RigisterVLLMRoutes(r *gin.Engine) {
	v1 := r.Group("/v1")
	v1.Use(middlewares.AuthMiddleware())
	v1.POST("/chat/completions", service.ChatCompletionsHandler())
	v1.Any("/:path", service.ProxyToVLLM())
	v1.Any("/:path/*any", service.ProxyToVLLM())
}
