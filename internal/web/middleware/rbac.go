package middleware

import (
	"net/http"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

var resellerAllowedPrefixes = []string{
	"panel/api/user/info",
	"panel/api/setting/all",
	"panel/api/setting/defaultSettings",
	"panel/api/server/status",
	"panel/api/server/getPanelUpdateInfo",
	"panel/api/inbounds/list",
	"panel/api/inbounds/options",
	"panel/api/inbounds/allLinks",
	"panel/api/inbounds/get/",
	"panel/api/clients/",
}

var resellerBlockedPrefixes = []string{
	"panel/api/inbounds/add",
	"panel/api/inbounds/del",
	"panel/api/inbounds/bulkDel",
	"panel/api/inbounds/update",
	"panel/api/inbounds/setEnable",
	"panel/api/inbounds/resetTraffic",
	"panel/api/inbounds/delAllClients",
	"panel/api/inbounds/resetAllTraffics",
	"panel/api/inbounds/import",
	"panel/api/inbounds/pushClientTraffics",
	"panel/api/setting/",
	"panel/api/server/",
	"panel/api/nodes/",
	"panel/api/hosts/",
	"panel/api/xray/",
	"panel/api/resellers/",
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

		for _, prefix := range resellerBlockedPrefixes {
			if strings.HasPrefix(path, prefix) {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
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
