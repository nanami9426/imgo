package service

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/models"
)

type GetUserListResp struct {
	Data []*models.UserBasic `json:"data"`
}

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

// @Summary 创建新用户
// @Tags users
// @Produce json
// @Success 200 {object} CreateUserResp
// @Router /user/create_user [get]
// @param user_name query string true "用户名"
// @param password query string true "密码"
// @param re_password query string true "确认密码"
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

// @Summary 删除用户
// @Tags users
// @Produce json
// @Success 200 {object} CreateUserResp
// @Router /user/del_user [get]
// @param user_id query string true "用户id"
func DeleteUser(c *gin.Context) {
	user := &models.UserBasic{}
	user_id, err := strconv.Atoi(c.Query("user_id"))
	if err != nil {
		c.JSON(200, gin.H{
			"message": "用户id格式错误",
		})
		return
	}
	user.ID = uint(user_id)
	models.DeleteUser(user)
	c.JSON(200, gin.H{
		"message": "删除成功",
	})
}

// @Summary 更新用户信息
// @Tags users
// @Produce json
// @Success 200 {object} CreateUserResp
// @Router /user/update_user [get]
// @param user_id query string true "用户id"
// @param user_name query string true "用户名"
func UpdateUser(c *gin.Context) {
	user := &models.UserBasic{}
	user_id, err := strconv.Atoi(c.Query("user_id"))
	if err != nil {
		c.JSON(200, gin.H{
			"message": "用户id格式错误",
		})
		return
	}
	user.ID = uint(user_id)
	user.Name = c.Query("user_name")
	err = models.UpdateUser(user)
	if err != nil {
		c.JSON(200, gin.H{
			"message": "修改失败",
			"err":     err,
		})
		return
	}
	c.JSON(200, gin.H{
		"message": "修改成功",
	})
}
