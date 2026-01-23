package service

import (
	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/models"
)

type GetUserListResp struct {
	Data []*models.UserBasic `json:"data"`
}

type CreateUserReq struct {
	UserName   string `json:"user_name" form:"user_name" binding:"required"`
	Password   string `json:"password" form:"password" binding:"required"`
	RePassword string `json:"re_password" form:"re_password" binding:"required"`
}

type DeleteUserReq struct {
	UserID uint `json:"user_id" form:"user_id" binding:"required"`
}

type UpdateUserReq struct {
	UserID   uint   `json:"user_id" form:"user_id" binding:"required"`
	UserName string `json:"user_name" form:"user_name" binding:"required"`
}

// @Summary 用户列表
// @Description 返回包含所有用户信息的列表
// @Tags users
// @Produce json
// @Success 200 {object} GetUserListResp
// @Router /user/user_list [post]
func GetUserList(c *gin.Context) {
	user_list, err := models.GetUserList()
	if err != nil {
		c.JSON(200, gin.H{
			"message": "获取用户列表失败",
			"err":     err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"data": user_list,
	})
}

type CreateUserResp struct {
	Message string `json:"message"`
}

// @Summary 创建新用户
// @Tags users
// @Produce json
// @Success 200 {object} CreateUserResp
// @Router /user/create_user [post]
// @param user_name formData string true "用户名"
// @param password formData string true "密码"
// @param re_password formData string true "确认密码"
func CreateUser(c *gin.Context) {
	req := &CreateUserReq{}
	if err := c.ShouldBind(req); err != nil {
		c.JSON(200, gin.H{
			"message": "参数错误",
			"err":     err.Error(),
		})
		return
	}
	user := &models.UserBasic{}
	user.Name = req.UserName
	password := req.Password
	re_password := req.RePassword
	if password != re_password {
		c.JSON(200, gin.H{
			"message": "两次输入的密码不一致",
		})
		return
	}
	user.Password = password
	if err := models.CreateUser(user); err != nil {
		c.JSON(200, gin.H{
			"message": "注册失败",
			"err":     err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"message": "注册成功",
	})
}

// @Summary 删除用户
// @Tags users
// @Produce json
// @Router /user/del_user [post]
// @param user_id formData int true "用户id"
func DeleteUser(c *gin.Context) {
	req := &DeleteUserReq{}
	if err := c.ShouldBind(req); err != nil {
		c.JSON(200, gin.H{
			"message": "用户id格式错误",
			"err":     err.Error(),
		})
		return
	}
	user := &models.UserBasic{}
	user.ID = req.UserID
	rows, err := models.DeleteUser(user)
	if err != nil {
		c.JSON(200, gin.H{
			"message": "删除失败",
			"err":     err.Error(),
		})
		return
	}
	if rows == 0 {
		c.JSON(200, gin.H{
			"message": "用户不存在或无需修改",
		})
		return
	}
	c.JSON(200, gin.H{
		"message": "删除成功",
	})
}

// @Summary 更新用户信息
// @Tags users
// @Produce json
// @Router /user/update_user [post]
// @param user_id formData int true "用户id"
// @param user_name formData string true "用户名"
func UpdateUser(c *gin.Context) {
	req := &UpdateUserReq{}
	if err := c.ShouldBind(req); err != nil {
		c.JSON(200, gin.H{
			"message": "参数错误",
			"err":     err.Error(),
		})
		return
	}
	user := &models.UserBasic{}
	user.ID = req.UserID
	user.Name = req.UserName
	rows, err := models.UpdateUser(user)
	if err != nil {
		c.JSON(200, gin.H{
			"message": "修改失败",
			"err":     err.Error(),
		})
		return
	}
	if rows == 0 {
		c.JSON(200, gin.H{
			"message": "用户不存在",
		})
		return
	}
	c.JSON(200, gin.H{
		"message": "修改成功",
	})
}
