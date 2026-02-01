package utils

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var v = viper.New()
var (
	DB  *gorm.DB
	RDB *redis.Client
	Ctx = context.Background()
)

func InitConfig() {
	v.SetConfigName("app")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	err := v.ReadInConfig()
	if err != nil {
		panic(err)
	}
}

func InitMySQL() {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=UTC",
		v.GetString("mysql.user"),
		v.GetString("mysql.password"),
		v.GetString("mysql.host"),
		v.GetInt("mysql.port"),
		v.GetString("mysql.db"),
	)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	DB = db
}

func InitRedis() {
	addr := fmt.Sprintf(
		"%s:%d",
		v.GetString("redis.host"),
		v.GetInt("redis.port"),
	)
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: v.GetString("redis.password"),
		DB:       v.GetInt("redis.db"),
	})
	_, err := rdb.Ping(Ctx).Result()
	if err != nil {
		panic("failed to connect redis: " + err.Error())
	}
	RDB = rdb
}
