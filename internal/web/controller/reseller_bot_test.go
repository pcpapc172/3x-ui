package controller

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
)

// TestResellerBotController_SetPlanAndPaymentMethod_FormEncoded reproduces a
// live regression: HttpUtil.post on the frontend sends
// application/x-www-form-urlencoded (see frontend/src/api/http-init.ts), but
// setPlan/setPaymentMethod originally bound straight into model.BotPlan /
// model.BotPaymentMethod, which only carry json tags. Gin's form binder
// silently left every field at its zero value, so saving always failed with
// "name is required" / "type must be card or crypto" no matter what the user
// entered. This drives a real form-encoded POST through the actual HTTP
// stack, the same way the browser does.
func TestResellerBotController_SetPlanAndPaymentMethod_FormEncoded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	db := database.GetDB()

	ib := &model.Inbound{Tag: "bot-test", Enable: true, Port: 6443, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	user := &model.User{Username: "owner1", Password: "x", Role: "admin"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	engine := gin.New()
	store := cookie.NewStore([]byte("reseller-bot-test-secret"))
	engine.Use(sessions.Sessions("3x-ui", store))
	engine.GET("/test-login", func(c *gin.Context) {
		if err := session.SetLoginUser(c, user); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})
	NewResellerBotController(engine.Group("/panel/api"))

	ts := httptest.NewServer(engine)
	defer ts.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	client := &http.Client{Jar: jar}

	loginResp, err := client.Get(ts.URL + "/test-login")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want 200", loginResp.StatusCode)
	}

	planForm := url.Values{
		"name":         {"30GB / 30 Days"},
		"trafficGB":    {"30"},
		"durationDays": {"30"},
		"priceToman":   {"250000"},
		"inboundIds":   {strconv.Itoa(ib.Id)},
		"enabled":      {"true"},
	}
	planResp, err := client.PostForm(ts.URL+"/panel/api/bot/plans/set", planForm)
	if err != nil {
		t.Fatalf("set plan: %v", err)
	}
	defer planResp.Body.Close()
	if planResp.StatusCode != http.StatusOK {
		t.Fatalf("set plan status = %d, want 200", planResp.StatusCode)
	}

	svc := &panel.BotService{}
	plans, err := svc.ListPlans(user.Id)
	if err != nil {
		t.Fatalf("ListPlans: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("plans after set = %d, want 1", len(plans))
	}
	if plans[0].Name != "30GB / 30 Days" {
		t.Fatalf("plan.Name = %q, want %q — form field did not bind (this is the reported bug)", plans[0].Name, "30GB / 30 Days")
	}
	if plans[0].TrafficGB != 30 || plans[0].DurationDays != 30 {
		t.Fatalf("plan traffic/duration = %d/%d, want 30/30", plans[0].TrafficGB, plans[0].DurationDays)
	}
	if len(plans[0].InboundIds) != 1 || plans[0].InboundIds[0] != ib.Id {
		t.Fatalf("plan.InboundIds = %v, want [%d]", plans[0].InboundIds, ib.Id)
	}

	methodForm := url.Values{
		"type":       {"card"},
		"label":      {"Bank Melli"},
		"value":      {"6037-0000-0000-0000"},
		"holderName": {"Ali R."},
		"enabled":    {"true"},
	}
	methodResp, err := client.PostForm(ts.URL+"/panel/api/bot/payment-methods/set", methodForm)
	if err != nil {
		t.Fatalf("set payment method: %v", err)
	}
	defer methodResp.Body.Close()
	if methodResp.StatusCode != http.StatusOK {
		t.Fatalf("set payment method status = %d, want 200", methodResp.StatusCode)
	}

	methods, err := svc.ListPaymentMethods(user.Id)
	if err != nil {
		t.Fatalf("ListPaymentMethods: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("payment methods after set = %d, want 1", len(methods))
	}
	if methods[0].Type != "card" {
		t.Fatalf("method.Type = %q, want %q — form field did not bind (this is the reported bug)", methods[0].Type, "card")
	}
	if methods[0].Label != "Bank Melli" || methods[0].Value != "6037-0000-0000-0000" {
		t.Fatalf("method label/value = %q/%q, want %q/%q", methods[0].Label, methods[0].Value, "Bank Melli", "6037-0000-0000-0000")
	}
}
