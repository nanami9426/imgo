package router

import (
	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/docs"
	"github.com/nanami9426/imgo/service"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func Router() *gin.Engine {
	r := gin.Default()
	docs.SwaggerInfo.BasePath = "/"
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	r.GET("/index", service.GetIndex)
	r.POST("/user/user_list", service.GetUserList)
	r.POST("/user/create_user", service.CreateUser)
	r.POST("/user/del_user", service.DeleteUser)
	r.POST("/user/update_user", service.UpdateUser)
	r.POST("/user/user_login", service.UserLogin)

	r.GET("/chat/send_message", service.SendMessage)

	return r
}
