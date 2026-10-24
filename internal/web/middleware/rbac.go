package middleware

import (
	"net/http"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

var resellerAllowedPrefixes = []string{
	"panel/api/user/info",
	"panel/api/user/overview",
	"panel/api/setting/all",
	"panel/api/setting/defaultSettings",
	"panel/api/server/status",
	"panel/api/server/getPanelUpdateInfo",
	"panel/api/inbounds/list",
	"panel/api/inbounds/options",
	"panel/api/inbounds/allLinks",
	"panel/api/inbounds/get/",
	"panel/api/clients/",
	"panel/api/bot/",
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
		basePath := c.GetString("base_path")
		if basePath != "" && basePath != "/" {
			path = strings.TrimPrefix(path, basePath)
		}
		path = strings.TrimPrefix(path, "/")

		// Client/inbound pages need share hosts and credential-free node metadata.
		// Permit only these exact reads; management endpoints remain admin-only.
		if c.Request.Method == http.MethodGet && (path == "panel/api/hosts/list" || path == "panel/api/nodes/list") {
			c.Next()
			return
		}

		for _, prefix := range resellerAllowedPrefixes {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
		}

		c.AbortWithStatus(http.StatusForbidden)
	}
}
