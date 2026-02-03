package utils

import (
	"context"
	"errors"
	"fmt"
)

// PublishToRedis 将消息发布到指定 Redis 频道。
func PublishToRedis(ctx context.Context, channel string, message string) error {
	if RDB == nil {
		return errors.New("redis not initialized")
	}
	fmt.Printf("PublishToRedis[%s]: %s\n", channel, message)
	return RDB.Publish(ctx, channel, message).Err()
}

// SubscribeFromRedis 订阅 Redis 频道并阻塞等待一条消息（适合一次性拉取）。
func SubscribeFromRedis(ctx context.Context, channel string) (string, error) {
	if RDB == nil {
		return "", errors.New("redis not initialized")
	}
	pubsub := RDB.Subscribe(ctx, channel)
	defer pubsub.Close()

	msg, err := pubsub.ReceiveMessage(ctx)
	fmt.Println("SubscribeFromRedis: ", msg)

	if err != nil {
		return "", err
	}
	return msg.Payload, err
}
