package router

import (
	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/docs"
	"github.com/nanami9426/imgo/internal/router/middlewares"
	"github.com/nanami9426/imgo/internal/service"
)

func Router() *gin.Engine {
	r := gin.Default()
	r.Use(middlewares.CORSMiddleware())
	r.Use(middlewares.APILoggingMiddleware())

	docs.SwaggerInfo.BasePath = "/"
	r.GET("/healthz", service.Healthz)

	RegisterSwagger(r)
	RegisterUserRoutes(r)
	RigisterChatRoutes(r)
	RigisterVLLMRoutes(r)
	RegisterUsageRoutes(r)
	return r
}
