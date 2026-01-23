package models

import (
	"time"

	"github.com/nanami9426/imgo/utils"
	"gorm.io/gorm"
)

type UserBasic struct {
	gorm.Model
	Name          string
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

func GetUserList() []*UserBasic {
	var user_list []*UserBasic
	utils.DB.Find(&user_list)
	return user_list
}

func CreateUser(user *UserBasic) error {
	return utils.DB.Create(user).Error
}

func DeleteUser(user *UserBasic) error {
	return utils.DB.Delete(user).Error
}

func UpdateUser(user *UserBasic) error {
	return utils.DB.Model(&UserBasic{}).Where("id=?", user.ID).Updates(user).Error
	// return utils.DB.Save(user).Error
}
