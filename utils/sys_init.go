package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var V = viper.New()
var (
	DB               *gorm.DB
	RDB              *redis.Client
	Ctx              = context.Background()
	WSPublishKey     string
	DefaultJWTSecret string
	DefaultJWTTTL    = 60 * time.Second
	TokenVersionMax  uint
)

func InitParam() {
	// redis_pubsub.go
	WSPublishKey = V.GetString("ws.public_channel")

	// auth.go
	DefaultJWTSecret = V.GetString("jwt.secret")
	TokenVersionMax = V.GetUint("token_version_max.n")
}

func InitConfig() {
	V.SetConfigName("app")
	V.SetConfigType("yaml")
	V.AddConfigPath("./config")
	err := V.ReadInConfig()
	if err != nil {
		panic(err)
	}
	InitParam()
	InitMySQL()
	InitRedis()
}

func InitMySQL() {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=UTC",
		V.GetString("mysql.user"),
		V.GetString("mysql.password"),
		V.GetString("mysql.host"),
		V.GetInt("mysql.port"),
		V.GetString("mysql.db"),
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
		V.GetString("redis.host"),
		V.GetInt("redis.port"),
	)
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: V.GetString("redis.password"),
		DB:       V.GetInt("redis.db"),
	})
	_, err := rdb.Ping(Ctx).Result()
	if err != nil {
		panic("failed to connect redis: " + err.Error())
	}
	RDB = rdb
}
