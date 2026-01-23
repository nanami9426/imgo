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
	r.GET("/user/user_list", service.GetUserList)
	r.GET("/user/create_user", service.CreateUser)
	r.GET("/user/del_user", service.DeleteUser)
	r.GET("/user/update_user", service.UpdateUser)
	return r
}
