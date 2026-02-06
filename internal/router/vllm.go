package router

import (
	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/internal/service"
)

func RigisterVLLMRoutes(r *gin.Engine) {
	v1 := r.Group("/v1")
	v1.Use(AuthMiddleware())
	v1.Any("/*any", service.ProxyToVLLM())
}
