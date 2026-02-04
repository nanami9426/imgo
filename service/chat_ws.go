package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/nanami9426/imgo/models"
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
	userID, err := getUserIDFromRequest(c)
	if err != nil {
		c.JSON(200, gin.H{
			"stat_code": utils.StatUnauthorized,
			"stat":      utils.StatText(utils.StatUnauthorized),
			"message":   "token或user_id无效",
			"err":       err.Error(),
		})
		return
	}

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

	if err = MessageHandler(ws, ctx, userID); err != nil {
		// WebSocket 已升级，不能再返回 HTTP JSON，只记录日志即可。
		fmt.Println("SendMessage error:", err)
	}
}

type WSMessageIn struct {
	ToID         int64  `json:"to_id"`
	MessageType  int    `json:"message_type,omitempty"`
	MessageMedia int    `json:"message_media,omitempty"`
	Content      string `json:"content"`
}

type WSMessageOut struct {
	MessageID    int64  `json:"message_id,omitempty"`
	FromID       int64  `json:"from_id"`
	ToID         int64  `json:"to_id"`
	MessageType  int    `json:"message_type,omitempty"`
	MessageMedia int    `json:"message_media,omitempty"`
	Content      string `json:"content"`
	Timestamp    string `json:"timestamp"`
}

func MessageHandler(ws *websocket.Conn, ctx context.Context, userID int64) error {
	/*
		goroutine读WS、写Redis
		主循环读Redis、写WS
	*/
	if utils.RDB == nil {
		return errors.New("redis not initialized")
	}
	// 订阅当前用户的 Redis 渠道，后续从此通道转发消息给 WS 客户端
	pubsub := utils.RDB.Subscribe(ctx, userChannel(userID))
	defer pubsub.Close()

	msgCh := pubsub.Channel()
	errCh := make(chan error, 1)

	// 读取 WS 客户端消息并发布到 Redis，便于转发给指定用户
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

			var in WSMessageIn
			if err := json.Unmarshal(msg, &in); err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
			if in.ToID <= 0 || strings.TrimSpace(in.Content) == "" {
				select {
				case errCh <- errors.New("invalid message payload"):
				default:
				}
				return
			}

			out := WSMessageOut{
				FromID:       userID,
				ToID:         in.ToID,
				MessageType:  in.MessageType,
				MessageMedia: in.MessageMedia,
				Content:      in.Content,
				Timestamp:    time.Now().UTC().Format(time.RFC3339Nano),
			}

			if utils.DB != nil {
				record := &models.ChatMessage{
					MessageID:    utils.GenerateID(),
					FromID:       userID,
					ToID:         in.ToID,
					MessageType:  in.MessageType,
					MessageMedia: in.MessageMedia,
					Content:      in.Content,
				}
				if err := utils.DB.Create(record).Error; err == nil {
					out.MessageID = record.MessageID
				} else {
					fmt.Println("save chat message error:", err)
				}
			}

			payload, err := json.Marshal(out)
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}

			// 将消息发给对方
			if err := utils.PublishToRedis(ctx, userChannel(in.ToID), string(payload)); err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}

			// 将消息发给自己
			if in.ToID != userID {
				if err := utils.PublishToRedis(ctx, userChannel(userID), string(payload)); err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}
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
			if err := ws.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
				return err
			}
		case err := <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func userChannel(userID int64) string {
	return fmt.Sprintf("%s:user:%d", utils.WSPublishKey, userID)
}

func getUserIDFromRequest(c *gin.Context) (int64, error) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		token = strings.TrimSpace(c.GetHeader("Authorization"))
	}
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	if token != "" {
		claims, err := utils.CheckToken(token, utils.JWTSecret())
		if err != nil {
			return 0, err
		}
		return int64(claims.UserID), nil
	}

	userIDStr := strings.TrimSpace(c.Query("user_id"))
	if userIDStr == "" {
		return 0, errors.New("missing token or user_id")
	}
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil || userID <= 0 {
		return 0, errors.New("invalid user_id")
	}
	return userID, nil
}
