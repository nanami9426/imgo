package utils

import (
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func CheckPassword(hashed string, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
	return err == nil
}

func GenerateUserID() int64 {
	node, _ := snowflake.NewNode(1)
	user_id := node.Generate().Int64()
	return user_id
}

type Claims struct {
	UserID uint   `json:"id"`
	Role   string `json:"role"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func GenerateToken(secret []byte, user_id uint, email string, ttl time.Duration) (string, error) {

	return "", nil
}
