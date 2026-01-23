package service

import (
	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/models"
)

type GetUserListResp struct {
	Data []*models.UserBasic `json:"data"`
}

// GetUserList
// @Summary 用户列表
// @Description 返回包含所有用户信息的列表
// @Tags users
// @Produce json
// @Success 200 {object} GetUserListResp
// @Router /user/user_list [get]
func GetUserList(c *gin.Context) {
	var user_list []*models.UserBasic
	user_list = models.GetUserList()
	c.JSON(200, gin.H{
		"data": user_list,
	})
}

type CreateUserResp struct {
	Data string `json:"data"`
}

// CreateUser
// @Summary 创建新用户
// @Tags users
// @Produce json
// @Success 200 {object} CreateUserResp
// @Router /user/create_user [get]
func CreateUser(c *gin.Context) {
	user := &models.UserBasic{}
	user.Name = c.Query("user_name")
	password := c.Query("password")
	re_password := c.Query("re_password")
	if password != re_password {
		c.JSON(200, gin.H{
			"message": "两次输入的密码不一致",
		})
		return
	}
	user.Password = password
	models.CreateUser(user)
	c.JSON(200, gin.H{
		"message": "注册成功",
	})
}
