package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/nanami9426/imgo/utils"
)

// ug 用于把 HTTP 请求升级为 WebSocket 连接。
var ug = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// @Summary 发送消息
// @Tags chat
// @Produce json
// @Router /chat/send_message [post]
func SendMessage(c *gin.Context) {
	// 将 HTTP 连接升级为 WebSocket
	// ws 在一次握手后建立长连接，服务端和客户端都可以随时发消息，不需要每次重新建连接。

	ws, err := ug.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(400, gin.H{
			"err":       err.Error(),
			"stat_code": utils.StatInternalError,
			"stat":      utils.StatText(utils.StatInternalError),
		})
		return
	}
	defer ws.Close()

	ctx := c.Request.Context()

	if err = MessageHandler(ws, ctx); err != nil {
		// WebSocket 已升级，不能再返回 HTTP JSON，只记录日志即可。
		fmt.Println("SendMessage error:", err)
	}
}

func MessageHandler(ws *websocket.Conn, ctx context.Context) error {
	if utils.RDB == nil {
		return errors.New("redis not initialized")
	}
	// 订阅 Redis 渠道，后续从此通道转发消息给 WS 客户端
	pubsub := utils.RDB.Subscribe(ctx, utils.WSPublishKey)
	defer pubsub.Close()

	msgCh := pubsub.Channel()
	errCh := make(chan error, 1)

	// 读取 WS 客户端消息并发布到 Redis，便于广播给其他订阅者
	go func() {
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
			if err := utils.PublishToRedis(ctx, utils.WSPublishKey, string(msg)); err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
		}
	}()

	for {
		select {
		case msg, ok := <-msgCh:
			if !ok {
				return nil
			}
			// 从 Redis 收到消息后写回 WS
			t := time.Now().Format("2006-01-02 15:04:05")
			formatedMessage := fmt.Sprintf("[%s]: %s", t, msg.Payload)
			fmt.Println("MessageHandler: ", formatedMessage)
			if err := ws.WriteMessage(websocket.TextMessage, []byte(formatedMessage)); err != nil {
				return err
			}
		case err := <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
