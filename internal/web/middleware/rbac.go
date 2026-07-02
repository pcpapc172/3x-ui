package middleware

import (
	"net/http"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

// resellerAllowedPrefixes lists API path prefixes a reseller may access.
var resellerAllowedPrefixes = []string{
	"/panel/api/inbounds/list",
	"/panel/api/inbounds/options",
	"/panel/api/inbounds/allLinks",
	"/panel/api/clients/",
	"/panel/api/user/info",
}

// resellerBlockedPrefixes lists API path prefixes a reseller is denied from.
// This is a safety net: if a new endpoint is added and not in the allow-list
// above, it is also blocked explicitly here.
var resellerBlockedPrefixes = []string{
	"/panel/api/setting/",
	"/panel/api/server/",
	"/panel/api/nodes/",
	"/panel/api/hosts/",
	"/panel/api/xray/",
	"/panel/api/resellers/",
}

// RBACMiddleware enforces role-based access control. Admin users have full
// access; reseller users are restricted to client management and their own
// inbound views. The middleware reads the logged-in user's role from the
// session (reloaded on every request) and blocks unauthorised paths.
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

		for _, prefix := range resellerBlockedPrefixes {
			if strings.HasPrefix(path, prefix) {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		allowed := false
		for _, prefix := range resellerAllowedPrefixes {
			if strings.HasPrefix(path, prefix) {
				allowed = true
				break
			}
		}
		if !allowed {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}
