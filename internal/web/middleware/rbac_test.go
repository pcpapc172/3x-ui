package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

func TestResellerPageMetadataAccess(t *testing.T) {
	for _, base := range []string{"/", "/private-panel/"} {
		for _, tc := range []struct {
			method, path string
			status       int
		}{
			{"GET", "panel/api/hosts/list", http.StatusNoContent},
			{"GET", "panel/api/nodes/list", http.StatusNoContent},
			{"GET", "panel/api/clients/list/paged", http.StatusNoContent},
			{"GET", "panel/api/inbounds/options", http.StatusNoContent},
			{"POST", "panel/api/hosts/list", http.StatusForbidden},
			{"GET", "panel/api/hosts/list/extra", http.StatusForbidden},
			{"GET", "panel/api/nodes/get/1", http.StatusForbidden},
			{"POST", "panel/api/hosts/add", http.StatusForbidden},
			{"POST", "panel/api/nodes/update/1", http.StatusForbidden},
			{"GET", "panel/api/resellers/list", http.StatusForbidden},
		} {
			t.Run(base+tc.method+tc.path, func(t *testing.T) {
				r := gin.New()
				r.Use(func(c *gin.Context) {
					c.Set("base_path", base)
					session.SetAPIAuthUser(c, &model.User{Id: 2, Role: "reseller"})
					c.Next()
				}, RBACMiddleware())
				r.NoRoute(func(c *gin.Context) { c.Status(http.StatusNoContent) })
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest(tc.method, base+tc.path, nil))
				if w.Code != tc.status {
					t.Fatalf("%s %s: HTTP %d, want %d", tc.method, base+tc.path, w.Code, tc.status)
				}
			})
		}
	}
}
