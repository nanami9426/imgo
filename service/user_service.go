package service

import (
	"github.com/asaskevich/govalidator"
	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/models"
	"github.com/nanami9426/imgo/utils"
)

type CreateUserReq struct {
	UserName   string `json:"user_name" form:"user_name" binding:"required"`
	Password   string `json:"password" form:"password" binding:"required"`
	RePassword string `json:"re_password" form:"re_password" binding:"required"`
	Email      string `json:"email" form:"email"`
}

type DeleteUserReq struct {
	UserID int64 `json:"user_id" form:"user_id" binding:"required"`
}

type UpdateUserReq struct {
	UserID   int64  `json:"user_id" form:"user_id" binding:"required"`
	UserName string `json:"user_name" form:"user_name"`
	Email    string `json:"email" form:"email"`
}

type UserLoginReq struct {
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}

// @Summary 用户列表
// @Description 返回包含所有用户信息的列表
// @Tags users
// @Produce json
// @Router /user/user_list [post]
func GetUserList(c *gin.Context) {
	user_list, err := models.GetUserList()
	if err != nil {
		c.JSON(200, gin.H{
			"stat_code": utils.StatDatabaseError,
			"stat":      utils.StatText(utils.StatDatabaseError),
			"message":   "获取用户列表失败",
			"err":       err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"stat_code": utils.StatSuccess,
		"stat":      utils.StatText(utils.StatSuccess),
		"data":      user_list,
	})
}

// @Summary 创建新用户
// @Tags users
// @Produce json
// @Router /user/create_user [post]
// @param user_name formData string true "用户名"
// @param password formData string true "密码"
// @param re_password formData string true "确认密码"
// @param email formData string false "邮箱"
func CreateUser(c *gin.Context) {
	req := &CreateUserReq{}
	if err := c.ShouldBind(req); err != nil {
		c.JSON(200, gin.H{
			"stat_code": utils.StatInvalidParam,
			"stat":      utils.StatText(utils.StatInvalidParam),
			"message":   "参数错误",
			"err":       err.Error(),
		})
		return
	}
	user := &models.UserBasic{}
	user.Name = req.UserName
	password := req.Password
	re_password := req.RePassword
	if password != re_password {
		c.JSON(200, gin.H{
			"stat_code": utils.StatInvalidParam,
			"stat":      utils.StatText(utils.StatInvalidParam),
			"message":   "两次输入的密码不一致",
		})
		return
	}
	user.Password, _ = utils.HashPassword(password)
	if !govalidator.IsEmail(req.Email) {
		c.JSON(200, gin.H{
			"stat_code": utils.StatInvalidParam,
			"stat":      utils.StatText(utils.StatInvalidParam),
			"message":   "邮箱格式错误",
		})
		return
	}
	if models.EmailIsExists(req.Email) {
		c.JSON(200, gin.H{
			"stat_code": utils.StatConflict,
			"stat":      utils.StatText(utils.StatConflict),
			"message":   "该邮箱已注册",
		})
		return
	}
	user.Email = req.Email
	user_id := utils.GenerateUserID()
	user.UserID = user_id
	if err := models.CreateUser(user); err != nil {
		c.JSON(200, gin.H{
			"stat_code": utils.StatDatabaseError,
			"stat":      utils.StatText(utils.StatDatabaseError),
			"message":   "注册失败",
			"err":       err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"stat_code": utils.StatSuccess,
		"stat":      utils.StatText(utils.StatSuccess),
		"message":   "注册成功",
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
			"stat_code": utils.StatInvalidParam,
			"stat":      utils.StatText(utils.StatInvalidParam),
			"message":   "参数错误",
			"err":       err.Error(),
		})
		return
	}
	user, rows := models.FindUserByUserID(req.UserID)
	if rows == 0 {
		c.JSON(200, gin.H{
			"stat_code": utils.StatNotFound,
			"stat":      utils.StatText(utils.StatNotFound),
			"message":   "用户不存在",
		})
		return
	}
	_, err := models.DeleteUser(&user)
	if err != nil {
		c.JSON(200, gin.H{
			"stat_code": utils.StatDatabaseError,
			"stat":      utils.StatText(utils.StatDatabaseError),
			"message":   "删除失败",
			"err":       err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"stat_code": utils.StatSuccess,
		"stat":      utils.StatText(utils.StatSuccess),
		"message":   "删除成功",
	})
}

// @Summary 更新用户信息
// @Tags users
// @Produce json
// @Router /user/update_user [post]
// @param user_id formData int true "用户id"
// @param user_name formData string false "用户名"
// @param email formData string false "邮箱"
func UpdateUser(c *gin.Context) {
	req := &UpdateUserReq{}
	if err := c.ShouldBind(req); err != nil {
		c.JSON(200, gin.H{
			"stat_code": utils.StatInvalidParam,
			"stat":      utils.StatText(utils.StatInvalidParam),
			"message":   "参数错误",
			"err":       err.Error(),
		})
		return
	}
	if !govalidator.IsEmail(req.Email) && "" != req.Email {
		c.JSON(200, gin.H{
			"stat_code": utils.StatInvalidParam,
			"stat":      utils.StatText(utils.StatInvalidParam),
			"message":   "邮箱格式错误",
		})
		return
	}
	data_update := map[string]interface{}{
		"UserID": req.UserID,
		"Name":   req.UserName,
	}
	if "" != req.Email {
		if models.EmailIsExists(req.Email) {
			c.JSON(200, gin.H{
				"stat_code": utils.StatConflict,
				"stat":      utils.StatText(utils.StatConflict),
				"message":   "该邮箱已注册",
			})
			return
		}
		data_update["Email"] = req.Email
	}
	rows, err := models.UpdateUser(data_update)
	if err != nil {
		c.JSON(200, gin.H{
			"stat_code": utils.StatDatabaseError,
			"stat":      utils.StatText(utils.StatDatabaseError),
			"message":   "修改失败",
			"err":       err.Error(),
		})
		return
	}
	if rows == 0 {
		c.JSON(200, gin.H{
			"stat_code": utils.StatNotFound,
			"stat":      utils.StatText(utils.StatNotFound),
			"message":   "用户不存在",
		})
		return
	}
	c.JSON(200, gin.H{
		"stat_code": utils.StatSuccess,
		"stat":      utils.StatText(utils.StatSuccess),
		"message":   "修改成功",
	})
}

// @Summary 用户登录
// @Tags users
// @Produce json
// @Router /user/user_login [post]
// @param email formData string true "邮箱"
// @param password formData string true "密码"
func UserLogin(c *gin.Context) {
	req := &UserLoginReq{}
	if err := c.ShouldBind(req); err != nil {
		c.JSON(200, gin.H{
			"stat_code": utils.StatInvalidParam,
			"stat":      utils.StatText(utils.StatInvalidParam),
			"message":   "参数错误",
			"err":       err.Error(),
		})
		return
	}
	if !govalidator.IsEmail(req.Email) || !models.EmailIsExists(req.Email) {
		c.JSON(200, gin.H{
			"stat_code": utils.StatInvalidParam,
			"stat":      utils.StatText(utils.StatInvalidParam),
			"message":   "邮箱格式有误或邮箱不存在",
		})
		return
	}
	user, _ := models.FindUserByEmail(req.Email)
	hashed_password := user.Password
	if !utils.CheckPassword(hashed_password, req.Password) {
		c.JSON(200, gin.H{
			"stat_code": utils.StatUnauthorized,
			"stat":      utils.StatText(utils.StatUnauthorized),
			"message":   "密码错误",
		})
		return
	}
	role := user.Identity
	if role == "" {
		role = "user"
	}

	version, err := utils.GetTokenVersion(c, uint(user.UserID))
	if err != nil {
		c.JSON(200, gin.H{
			"stat_code": utils.StatInternalError,
			"stat":      utils.StatText(utils.StatInternalError),
			"err":       err.Error(),
		})
		return
	}
	version = (version + 1) % utils.TokenVersionMax

	token, err := utils.GenerateToken(utils.JWTSecret(), uint(user.UserID), role, utils.JWTTTL(), version)
	if err != nil {
		c.JSON(200, gin.H{
			"stat_code": utils.StatInternalError,
			"stat":      utils.StatText(utils.StatInternalError),
			"message":   "生成token失败",
			"err":       err.Error(),
		})
		return
	}

	_, err = utils.IncrTokenVersion(c, uint(user.UserID))
	if err != nil {
		c.JSON(200, gin.H{
			"stat_code": utils.StatInternalError,
			"stat":      utils.StatText(utils.StatInternalError),
			"err":       err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"stat_code": utils.StatSuccess,
		"stat":      utils.StatText(utils.StatSuccess),
		"token":     token,
		"version":   version,
		"user_id":   user.UserID,
	})
}
