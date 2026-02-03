package utils

import (
	"context"
	"errors"
	"fmt"
)

var (
	WSPublishKey = V.GetString("ws.publish_key")
)

func PublishToRedis(ctx context.Context, channel string, message string) error {
	if RDB == nil {
		return errors.New("redis not initialized")
	}
	fmt.Println("PublishToRedis: ", message)
	return RDB.Publish(ctx, channel, message).Err()
}

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
