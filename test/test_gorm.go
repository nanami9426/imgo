package main

// 运行这个来生成表

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/nanami9426/imgo/internal/models"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func get_dsn() string {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(".env文件读取失败")
	}
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=UTC",
		os.Getenv("MYSQL_USER"),
		os.Getenv("MYSQL_PASSWORD"),
		os.Getenv("MYSQL_HOST"),
		os.Getenv("MYSQL_PORT"),
		os.Getenv("MYSQL_DB"),
	)
	return dsn
}

func main() {
	db, err := gorm.Open(mysql.Open(get_dsn()), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	db.AutoMigrate(&models.UserBasic{})
	db.AutoMigrate(&models.ChatMessage{})
	db.AutoMigrate(&models.Group{})
	db.AutoMigrate(&models.UserRelationship{})
	db.AutoMigrate(&models.APIUsage{})
}
