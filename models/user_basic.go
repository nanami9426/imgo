package models

import (
	"time"

	"github.com/nanami9426/imgo/utils"
	"gorm.io/gorm"
)

type UserBasic struct {
	gorm.Model
	Name          string
	Password      string `json:"-"`
	Phone         string
	Email         string `valid:"email"`
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

func GetUserList() ([]*UserBasic, error) {
	var user_list []*UserBasic
	result := utils.DB.Find(&user_list)
	return user_list, result.Error
}

func CreateUser(user *UserBasic) error {
	return utils.DB.Create(user).Error
}

func DeleteUser(user *UserBasic) (int64, error) {
	result := utils.DB.Delete(user)
	return result.RowsAffected, result.Error
}

func UpdateUser(user *UserBasic) (int64, error) {
	// result := utils.DB.Model(&UserBasic{}).Where("id=?", user.ID).Update("name", user.Name)
	result := utils.DB.Model(&UserBasic{}).Where("id=?", user.ID).Updates(user)
	return result.RowsAffected, result.Error
}
