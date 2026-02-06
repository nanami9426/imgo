package router

import (
	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/internal/service"
)

func RigisterChatRoutes(r *gin.Engine) {
	chat := r.Group("/chat")
	{
		chat.GET("/sent_message", service.SendMessage)
	}
}
