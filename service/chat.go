package service

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/nanami9426/imgo/utils"
)

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

	err = MessageHandler(ws, c)
	if err != nil {
		c.JSON(400, gin.H{
			"err":       err.Error(),
			"stat_code": utils.StatInternalError,
			"stat":      utils.StatText(utils.StatInternalError),
		})
		return
	}
}

func MessageHandler(ws *websocket.Conn, c *gin.Context) error {
	for {
		message, err := utils.SubscribeFromRedis(c, utils.WSPublishKey)
		if err != nil {
			return err
		}
		t := time.Now().Format("2006-01-02 15:04:05")
		formated_message := fmt.Sprintf("[%s]: %s", t, message)
		fmt.Println("MessageHandler: ", formated_message)
		err = ws.WriteMessage(websocket.TextMessage, []byte(formated_message)) // websocket.TextMessage == 1 代表字符串
		if err != nil {
			return err
		}
	}
}
