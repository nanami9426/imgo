package middlewares

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nanami9426/imgo/internal/utils"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(c.GetHeader("Authorization"))
		if strings.HasPrefix(strings.ToLower(token), "bearer ") {
			token = strings.TrimSpace(token[7:])
		}
		if token == "" {
			token = strings.TrimSpace(c.Query("token"))
		}
		/*
			取 token只从header、query取。
			在网关场景不要使用_ = c.ShouldBind(req)，会读掉request body，
			导致后面再转发请求时body可能已经被消费了。
		*/
		if token == "" {
			utils.Abort(c, http.StatusUnauthorized, utils.StatUnauthorized, "token不能为空", nil)
			return
		}

		claims, err := utils.CheckToken(token, utils.JWTSecret())
		if err != nil {
			utils.Abort(c, http.StatusUnauthorized, utils.StatUnauthorized, "token无效或已过期", err)
			return
		}

		latestVersion, _ := utils.GetTokenVersion(c, claims.UserID)
		diff := func(a, b uint) uint {
			if a >= b {
				return a - b
			}
			return b - a
		}(latestVersion, claims.Version)

		if diff >= utils.LoginDeviceMax {
			utils.Abort(c, http.StatusUnauthorized, utils.StatUnauthorized, "登录设备达到上限", nil)
			return
		}

		// c.Set("claims", claims)
		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		c.Next()
	}
}
