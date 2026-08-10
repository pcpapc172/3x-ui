package panel

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func setupBotTestDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func createTestInbound(t *testing.T, tag string) *model.Inbound {
	t.Helper()
	ib := &model.Inbound{
		Tag:            tag,
		Enable:         true,
		Listen:         "0.0.0.0",
		Port:           0,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
		Settings:       `{"clients":[]}`,
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound %s: %v", tag, err)
	}
	return ib
}

func TestApproveOrder_RejectsDisallowedInbound(t *testing.T) {
	setupBotTestDB(t)
	db := database.GetDB()

	allowed := createTestInbound(t, "allowed")
	disallowed := createTestInbound(t, "disallowed")

	reseller := &model.User{
		Username:            "reseller1",
		Role:                "reseller",
		AllowedInboundsMode: "select",
		AllowedInboundIds:   []int{allowed.Id},
	}
	if err := db.Create(reseller).Error; err != nil {
		t.Fatalf("create reseller: %v", err)
	}

	plan := &model.BotPlan{
		OwnerId:      reseller.Id,
		Name:         "test plan",
		TrafficGB:    10,
		DurationDays: 30,
		PriceToman:   100000,
		InboundIds:   []int{disallowed.Id},
		Enabled:      true,
	}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("create plan: %v", err)
	}

	order := &model.BotOrder{
		OwnerId: reseller.Id,
		PlanId:  plan.Id,
		Status:  model.BotOrderPendingReview,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}

	svc := &BotService{}
	if _, _, err := svc.ApproveOrder(order.Id, reseller.Id); err == nil {
		t.Fatal("ApproveOrder succeeded for a plan referencing an inbound outside the reseller's allowed set, want error")
	}

	var reloaded model.BotOrder
	if err := db.First(&reloaded, order.Id).Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if reloaded.Status != model.BotOrderPendingReview {
		t.Fatalf("order status = %q after rejected approval, want unchanged %q", reloaded.Status, model.BotOrderPendingReview)
	}

	var clientCount int64
	db.Model(&model.ClientRecord{}).Where("owner_id = ?", reseller.Id).Count(&clientCount)
	if clientCount != 0 {
		t.Fatalf("client count = %d after rejected approval, want 0 — no client should be provisioned", clientCount)
	}
}

func TestApproveOrder_Success(t *testing.T) {
	setupBotTestDB(t)
	db := database.GetDB()

	ib := createTestInbound(t, "allowed")

	reseller := &model.User{
		Username:            "reseller1",
		Role:                "reseller",
		AllowedInboundsMode: "select",
		AllowedInboundIds:   []int{ib.Id},
	}
	if err := db.Create(reseller).Error; err != nil {
		t.Fatalf("create reseller: %v", err)
	}

	plan := &model.BotPlan{
		OwnerId:      reseller.Id,
		Name:         "test plan",
		TrafficGB:    10,
		DurationDays: 30,
		PriceToman:   100000,
		InboundIds:   []int{ib.Id},
		Enabled:      true,
	}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("create plan: %v", err)
	}

	order := &model.BotOrder{
		OwnerId: reseller.Id,
		PlanId:  plan.Id,
		Status:  model.BotOrderPendingReview,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}

	svc := &BotService{}
	updated, email, err := svc.ApproveOrder(order.Id, reseller.Id)
	if err != nil {
		t.Fatalf("ApproveOrder: %v", err)
	}
	if updated.Status != model.BotOrderApproved {
		t.Fatalf("order status = %q, want %q", updated.Status, model.BotOrderApproved)
	}
	if email == "" {
		t.Fatal("ApproveOrder returned an empty client email")
	}

	client := &model.ClientRecord{}
	if err := db.Where("email = ?", email).First(client).Error; err != nil {
		t.Fatalf("provisioned client not found: %v", err)
	}
	if client.OwnerId != reseller.Id {
		t.Fatalf("client.OwnerId = %d, want %d", client.OwnerId, reseller.Id)
	}
	wantBytes := plan.TrafficGB * bytesPerGB
	if client.TotalGB != wantBytes {
		t.Fatalf("client.TotalGB = %d, want %d", client.TotalGB, wantBytes)
	}
	wantExpiry := -(int64(plan.DurationDays) * msPerDay)
	if client.ExpiryTime != wantExpiry {
		t.Fatalf("client.ExpiryTime = %d, want %d (negative = start-after-first-use duration)", client.ExpiryTime, wantExpiry)
	}
}
