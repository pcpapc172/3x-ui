package service

import (
	"encoding/json"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// seedResellerClient wires up one reseller-owned client the way production
// stores it: a ClientRecord, its client_inbounds link, a client_traffics row,
// and an entry in the local inbound's Settings JSON (what xray-core reads).
// Rows are created enabled then flipped off explicitly when enable is false —
// GORM's Create skips a zero-value bool and ClientRecord.Enable defaults true.
func seedResellerClient(t *testing.T, db *gorm.DB, ib *model.Inbound, resellerId int, email string, enable, locked bool) {
	t.Helper()
	var settings struct {
		Clients []map[string]any `json:"clients"`
	}
	if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
		t.Fatalf("unmarshal inbound settings: %v", err)
	}
	settings.Clients = append(settings.Clients, map[string]any{"email": email, "enable": enable})
	raw, _ := json.Marshal(settings)
	if err := db.Model(ib).Update("settings", string(raw)).Error; err != nil {
		t.Fatalf("update inbound settings: %v", err)
	}
	ib.Settings = string(raw)

	rec := &model.ClientRecord{Email: email, SubID: email, Enable: true, Locked: locked, OwnerId: resellerId}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("create client record %q: %v", email, err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("link client %q: %v", email, err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: email, Enable: true, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("create traffic %q: %v", email, err)
	}
	if !enable {
		if err := db.Model(&model.ClientRecord{}).Where("email = ?", email).Update("enable", false).Error; err != nil {
			t.Fatalf("disable client record %q: %v", email, err)
		}
		if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", email).Update("enable", false).Error; err != nil {
			t.Fatalf("disable client traffic %q: %v", email, err)
		}
	}
}

func reloadUser(t *testing.T, db *gorm.DB, id int) model.User {
	t.Helper()
	var u model.User
	if err := db.First(&u, id).Error; err != nil {
		t.Fatalf("reload user %d: %v", id, err)
	}
	return u
}

func reloadClientRecord(t *testing.T, db *gorm.DB, email string) model.ClientRecord {
	t.Helper()
	var r model.ClientRecord
	if err := db.Where("email = ?", email).First(&r).Error; err != nil {
		t.Fatalf("reload client record %q: %v", email, err)
	}
	return r
}

func inboundJSONClientEnable(t *testing.T, db *gorm.DB, inboundId int, email string) bool {
	t.Helper()
	var ib model.Inbound
	if err := db.First(&ib, inboundId).Error; err != nil {
		t.Fatalf("load inbound %d: %v", inboundId, err)
	}
	var settings struct {
		Clients []map[string]any `json:"clients"`
	}
	if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
		t.Fatalf("unmarshal inbound %d settings: %v", inboundId, err)
	}
	for _, c := range settings.Clients {
		if e, _ := c["email"].(string); e == email {
			en, _ := c["enable"].(bool)
			return en
		}
	}
	t.Fatalf("client %q not in inbound %d settings", email, inboundId)
	return false
}

// TestDisableResellerOverQuota_LocksAccountAndEnabledClients is the regression
// test for the reported bug: a reseller who blew past their usage limit kept
// full panel access and their clients kept serving traffic. The over-quota job
// must now (a) lock the reseller's own login, (b) disable + lock every client
// that was still enabled, and (c) leave a client the reseller had already
// disabled themselves untouched (enable=false, locked=false).
func TestDisableResellerOverQuota_LocksAccountAndEnabledClients(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	reseller := &model.User{
		Username: "r-overquota", Password: "x", Role: "reseller",
		UsageLimit: 100, UsageUp: 100, UsageDown: 0, LoginEpoch: 0,
	}
	if err := db.Create(reseller).Error; err != nil {
		t.Fatalf("create reseller: %v", err)
	}
	ib := &model.Inbound{
		UserId: 1, Tag: "rq-in", Enable: true, Port: 45011,
		Protocol: model.VLESS, Settings: `{"clients":[]}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	seedResellerClient(t, db, ib, reseller.Id, "rq-live@x", true, false)
	seedResellerClient(t, db, ib, reseller.Id, "rq-selfdisabled@x", false, false)

	_, count, err := svc.disableResellerOverQuotaClients(db, newTrafficMutationBatch())
	if err != nil {
		t.Fatalf("disableResellerOverQuotaClients: %v", err)
	}
	if count != 1 {
		t.Fatalf("disabled count = %d, want 1 (only the still-enabled client)", count)
	}

	if got := reloadUser(t, db, reseller.Id).LoginEpoch; got >= 0 {
		t.Fatalf("reseller LoginEpoch = %d, want negative (login locked)", got)
	}

	live := reloadClientRecord(t, db, "rq-live@x")
	if live.Enable || !live.Locked {
		t.Fatalf("over-quota client rq-live@x: enable=%v locked=%v, want enable=false locked=true", live.Enable, live.Locked)
	}
	if inboundJSONClientEnable(t, db, ib.Id, "rq-live@x") {
		t.Fatalf("over-quota client rq-live@x still enabled in inbound settings JSON, want disabled")
	}

	self := reloadClientRecord(t, db, "rq-selfdisabled@x")
	if self.Enable || self.Locked {
		t.Fatalf("self-disabled client: enable=%v locked=%v, want enable=false locked=false (untouched)", self.Enable, self.Locked)
	}
}

// TestDisableResellerOverQuota_UnderLimitIsNoop guards the predicate: a reseller
// still within their limit must not have their login or clients touched.
func TestDisableResellerOverQuota_UnderLimitIsNoop(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	reseller := &model.User{
		Username: "r-under", Password: "x", Role: "reseller",
		UsageLimit: 1000, UsageUp: 10, UsageDown: 10, LoginEpoch: 0,
	}
	if err := db.Create(reseller).Error; err != nil {
		t.Fatalf("create reseller: %v", err)
	}
	ib := &model.Inbound{
		UserId: 1, Tag: "ru-in", Enable: true, Port: 45012,
		Protocol: model.VLESS, Settings: `{"clients":[]}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	seedResellerClient(t, db, ib, reseller.Id, "ru-live@x", true, false)

	_, count, err := svc.disableResellerOverQuotaClients(db, newTrafficMutationBatch())
	if err != nil {
		t.Fatalf("disableResellerOverQuotaClients: %v", err)
	}
	if count != 0 {
		t.Fatalf("disabled count = %d, want 0", count)
	}
	if got := reloadUser(t, db, reseller.Id).LoginEpoch; got != 0 {
		t.Fatalf("reseller LoginEpoch = %d, want 0 (untouched)", got)
	}
	if !reloadClientRecord(t, db, "ru-live@x").Enable {
		t.Fatalf("client disabled while reseller under limit, want still enabled")
	}
}

// TestDisableResellerOverQuota_KeepsManualAccountDisable: an over-quota reseller
// the owner had already disabled by hand (LoginEpoch -1) must keep that exact
// value, so a later usage reset (which only clears LoginEpoch < -1) does not
// silently undo the owner's manual disable.
func TestDisableResellerOverQuota_KeepsManualAccountDisable(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	reseller := &model.User{
		Username: "r-manual", Password: "x", Role: "reseller",
		UsageLimit: 100, UsageUp: 100, LoginEpoch: -1,
	}
	if err := db.Create(reseller).Error; err != nil {
		t.Fatalf("create reseller: %v", err)
	}
	ib := &model.Inbound{
		UserId: 1, Tag: "rm-in", Enable: true, Port: 45013,
		Protocol: model.VLESS, Settings: `{"clients":[]}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	seedResellerClient(t, db, ib, reseller.Id, "rm-live@x", true, false)

	if _, _, err := svc.disableResellerOverQuotaClients(db, newTrafficMutationBatch()); err != nil {
		t.Fatalf("disableResellerOverQuotaClients: %v", err)
	}
	if got := reloadUser(t, db, reseller.Id).LoginEpoch; got != -1 {
		t.Fatalf("manually-disabled reseller LoginEpoch = %d, want -1 (unchanged)", got)
	}
}
