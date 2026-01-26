package models

import (
	"time"

	"github.com/nanami9426/imgo/utils"
	"gorm.io/gorm"
)

type UserBasic struct {
	gorm.Model
	Name          string
	UserID        int64 `gorm:"uniqueIndex"`
	Password      string
	Phone         string
	Email         string
	Identity      string
	ClientIP      string
	ClientPort    string
	LoginTime     *time.Time
	HeartbeatTime *time.Time
	LogoutTime    *time.Time
	IsLogout      bool
	DeviceInfo    string
}

func (u *UserBasic) TableName() string {

	return "user_basic"
}

// 获取所有用户列表
func GetUserList() ([]*UserBasic, error) {
	var user_list []*UserBasic
	result := utils.DB.Find(&user_list)
	return user_list, result.Error
}

// 创建新用户
func CreateUser(user *UserBasic) error {
	return utils.DB.Create(user).Error
}

// 逻辑删除用户
func DeleteUser(user *UserBasic) (int64, error) {
	result := utils.DB.Delete(user)
	return result.RowsAffected, result.Error
}

// 更新用户信息
func UpdateUser(data map[string]interface{}) (int64, error) {
	result := utils.DB.Model(&UserBasic{}).Where("id=?", data["ID"]).Updates(data)
	return result.RowsAffected, result.Error
}

// 判断邮箱是否存在
func EmailIsExists(email string) bool {
	result := utils.DB.Where("email = ?", email).First(&UserBasic{})
	return result.RowsAffected > 0
}

func FindUserByEmail(email string) (UserBasic, int64) {
	var user UserBasic
	result := utils.DB.Where("email = ?", email).First(&user)
	return user, result.RowsAffected
}

func FindUserByUserID(user_id int64) (UserBasic, int64) {
	var user UserBasic
	result := utils.DB.Where("user_id = ?", user_id).First(&user)
	return user, result.RowsAffected
}
