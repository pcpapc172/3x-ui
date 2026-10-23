package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestResellerUpdateDoesNotClearOmittedTerabyteQuota(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	const quota int64 = 3 * 1024 * 1024 * 1024 * 1024
	user := &model.User{Username: "quota-owner", Role: "reseller", UsageLimit: quota}
	if err := database.GetDB().Create(user).Error; err != nil {
		t.Fatalf("seed reseller: %v", err)
	}
	for i, body := range []string{`{"username":"renamed","multiplier":2}`, `{"username":"renamed","usageLimit":-1}`} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(user.Id)}}
		c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		(&ResellerController{}).update(c)
		var msg struct {
			Success bool `json:"success"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &msg); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		var got model.User
		if err := database.GetDB().First(&got, user.Id).Error; err != nil {
			t.Fatalf("reload reseller: %v", err)
		}
		if got.UsageLimit != quota {
			t.Fatalf("request %s changed 3 TiB quota to %d", body, got.UsageLimit)
		}
		if msg.Success != (i == 0) {
			t.Fatalf("request %s success=%v, want %v", body, msg.Success, i == 0)
		}
	}
}
