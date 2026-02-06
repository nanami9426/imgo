package service

import (
	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/internal/utils"
)

type GetIndexResp struct {
	Message string `json:"message" example:"hello"`
}

// GetIndex
// @Summary 测试路由
// @Tags example
// @Produce json
// @Success 200 {object} GetIndexResp
// @Router /healthz [get]
func Healthz(c *gin.Context) {
	utils.Success(c, gin.H{"message": "hello"})
}
