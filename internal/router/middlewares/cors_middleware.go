package middlewares

import (
	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/internal/utils"
)

/*
	普通业务handler：用Fail+return。
	中间件/鉴权：必须用Abort，确保后面的handler不会继续跑。
*/

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			if _, ok := utils.AllowedOrigins[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type")
			}
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
