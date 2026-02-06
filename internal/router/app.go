package router

import (
	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/docs"
	"github.com/nanami9426/imgo/internal/service"
)

func Router() *gin.Engine {
	r := gin.Default()
	r.Use(CORSMiddleware())
	docs.SwaggerInfo.BasePath = "/"
	r.GET("/index", service.GetIndex)

	RegisterSwagger(r)
	RegisterUserRoutes(r)
	RigisterChatRoutes(r)
	return r
}
