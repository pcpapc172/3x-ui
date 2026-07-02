package middleware

import (
	"net/http"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

var resellerAllowedPrefixes = []string{
	"/panel/api/user/info",
	"/panel/api/setting/all",
	"/panel/api/server/status",
	"/panel/api/inbounds/",
	"/panel/api/clients/",
}

func RBACMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := session.GetLoginUser(c)
		if user == nil {
			c.Next()
			return
		}
		if user.Role != "reseller" {
			c.Next()
			return
		}

		path := c.Request.URL.Path

		for _, prefix := range resellerAllowedPrefixes {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
		}

		c.AbortWithStatus(http.StatusForbidden)
	}
}
