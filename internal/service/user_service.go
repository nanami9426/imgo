package service

import (
	"net/http"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/internal/models"
	"github.com/nanami9426/imgo/internal/utils"
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

type CheckTokenReq struct {
	Token string `json:"token" form:"token"`
}

// @Summary 用户列表
// @Description 返回包含所有用户信息的列表
// @Tags users
// @Produce json
// @Router /user/user_list [post]
func GetUserList(c *gin.Context) {
	user_list, err := models.GetUserList()
	if err != nil {
		utils.Fail(c, http.StatusOK, utils.StatDatabaseError, "获取用户列表失败", err)
		return
	}
	utils.Success(c, user_list)
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
		utils.Fail(c, http.StatusOK, utils.StatInvalidParam, "参数错误", err)
		return
	}
	user := &models.UserBasic{}
	user.Name = req.UserName
	password := req.Password
	re_password := req.RePassword
	if password != re_password {
		utils.Fail(c, http.StatusOK, utils.StatInvalidParam, "两次输入的密码不一致", nil)
		return
	}
	user.Password, _ = utils.HashPassword(password)
	if !govalidator.IsEmail(req.Email) {
		utils.Fail(c, http.StatusOK, utils.StatInvalidParam, "邮箱格式错误", nil)
		return
	}
	if models.EmailIsExists(req.Email) {
		utils.Fail(c, http.StatusOK, utils.StatConflict, "该邮箱已注册", nil)
		return
	}
	user.Email = req.Email
	user_id := utils.GenerateUserID()
	user.UserID = user_id
	if err := models.CreateUser(user); err != nil {
		utils.Fail(c, http.StatusOK, utils.StatDatabaseError, "注册失败", err)
		return
	}
	utils.SuccessMessage(c, "注册成功")
}

// @Summary 删除用户
// @Tags users
// @Produce json
// @Router /user/del_user [post]
// @param user_id formData int true "用户id"
func DeleteUser(c *gin.Context) {
	req := &DeleteUserReq{}
	if err := c.ShouldBind(req); err != nil {
		utils.Fail(c, http.StatusOK, utils.StatInvalidParam, "参数错误", err)
		return
	}
	user, rows := models.FindUserByUserID(req.UserID)
	if rows == 0 {
		utils.Fail(c, http.StatusOK, utils.StatNotFound, "用户不存在", nil)
		return
	}
	_, err := models.DeleteUser(&user)
	if err != nil {
		utils.Fail(c, http.StatusOK, utils.StatDatabaseError, "删除失败", err)
		return
	}

	utils.SuccessMessage(c, "删除成功")
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
		utils.Fail(c, http.StatusOK, utils.StatInvalidParam, "参数错误", err)
		return
	}
	if !govalidator.IsEmail(req.Email) && "" != req.Email {
		utils.Fail(c, http.StatusOK, utils.StatInvalidParam, "邮箱格式错误", nil)
		return
	}
	data_update := map[string]interface{}{
		"UserID": req.UserID,
		"Name":   req.UserName,
	}
	if "" != req.Email {
		if models.EmailIsExists(req.Email) {
			utils.Fail(c, http.StatusOK, utils.StatConflict, "该邮箱已注册", nil)
			return
		}
		data_update["Email"] = req.Email
	}
	rows, err := models.UpdateUser(data_update)
	if err != nil {
		utils.Fail(c, http.StatusOK, utils.StatDatabaseError, "修改失败", err)
		return
	}
	if rows == 0 {
		utils.Fail(c, http.StatusOK, utils.StatNotFound, "用户不存在", nil)
		return
	}
	utils.SuccessMessage(c, "修改成功")
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
		utils.Fail(c, http.StatusOK, utils.StatInvalidParam, "参数错误", err)
		return
	}
	if !govalidator.IsEmail(req.Email) || !models.EmailIsExists(req.Email) {
		utils.Fail(c, http.StatusOK, utils.StatInvalidParam, "邮箱格式有误或邮箱不存在", nil)
		return
	}
	user, _ := models.FindUserByEmail(req.Email)
	hashed_password := user.Password
	if !utils.CheckPassword(hashed_password, req.Password) {
		utils.Fail(c, http.StatusOK, utils.StatUnauthorized, "密码错误", nil)
		return
	}
	role := user.Identity
	if role == "" {
		role = "user"
	}

	version, err := utils.GetTokenVersion(c, uint(user.UserID))
	if err != nil {
		utils.Fail(c, http.StatusOK, utils.StatInternalError, "内部错误", err)
		return
	}
	version = (version + 1) % utils.TokenVersionMax

	token, err := utils.GenerateToken(utils.JWTSecret(), uint(user.UserID), role, utils.JWTTTL(), version)
	if err != nil {
		utils.Fail(c, http.StatusOK, utils.StatInternalError, "生成token失败", err)
		return
	}

	_, err = utils.IncrTokenVersion(c, uint(user.UserID))
	if err != nil {
		utils.Fail(c, http.StatusOK, utils.StatInternalError, "内部错误", err)
		return
	}

	utils.Success(c, gin.H{
		"token":   token,
		"version": version,
		"user_id": user.UserID,
	})
}

// @Summary 校验 token 是否有效
// @Tags users
// @Produce json
// @Router /user/check_token [post]
// @param token header string false "Bearer token"
// @param token formData string false "token"
func CheckToken(c *gin.Context) {
	token := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	if token == "" {
		token = strings.TrimSpace(c.Query("token"))
	}
	if token == "" {
		req := &CheckTokenReq{}
		_ = c.ShouldBind(req)
		token = strings.TrimSpace(req.Token)
	}
	if token == "" {
		utils.Fail(c, http.StatusUnauthorized, utils.StatInvalidParam, "token不能为空", nil)
		return
	}
	uintDiff := func(a, b uint) uint {
		if a >= b {
			return a - b
		}
		return b - a
	}
	claims, err := utils.CheckToken(token, utils.JWTSecret())

	if err != nil {
		utils.Fail(c, http.StatusUnauthorized, utils.StatUnauthorized, "token无效或已过期", err)
		return
	}

	latest_version, _ := utils.GetTokenVersion(c, claims.UserID)
	diff := uintDiff(latest_version, claims.Version)
	if diff >= utils.LoginDeviceMax {
		utils.Fail(c, http.StatusUnauthorized, utils.StatUnauthorized, "登录设备达到上限", nil)
		return
	}

	exp := int64(0)
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Unix()
	}
	utils.Success(c, gin.H{
		"user_id": claims.UserID,
		"role":    claims.Role,
		"exp":     exp,
	})
}
