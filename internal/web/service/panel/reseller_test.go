package panel

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// seedResellerWithClient creates a reseller and one client of theirs,
// attached to a real inbound whose Settings JSON embeds the client (so
// ClientService.BulkSetEnable — the mechanism ToggleEnableReseller must
// cascade through — can actually find and toggle it, matching how a real
// client is represented on disk).
func seedResellerWithClient(t *testing.T, email string) (reseller *model.User, inbound *model.Inbound) {
	t.Helper()
	db := database.GetDB()

	reseller = &model.User{Username: "reseller-" + email, Password: "x", Role: "reseller"}
	if err := db.Create(reseller).Error; err != nil {
		t.Fatalf("create reseller: %v", err)
	}

	client := model.Client{
		Email:      email,
		ID:         "11111111-1111-1111-1111-111111111111",
		SubID:      email,
		Enable:     true,
		TotalGB:    0,
		ExpiryTime: 0,
	}
	settingsJSON, err := json.Marshal(map[string][]model.Client{"clients": {client}})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	inbound = &model.Inbound{
		Tag:            "reseller-toggle-" + email,
		Enable:         true,
		Listen:         "0.0.0.0",
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
		Settings:       string(settingsJSON),
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	rec := &model.ClientRecord{
		Email:   email,
		UUID:    client.ID,
		SubID:   email,
		Enable:  true,
		OwnerId: reseller.Id,
	}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("create client record: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: inbound.Id}).Error; err != nil {
		t.Fatalf("create client-inbound link: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: email, Enable: true, InboundId: inbound.Id}).Error; err != nil {
		t.Fatalf("create client traffic: %v", err)
	}

	return reseller, inbound
}

// addClientToReseller attaches one more client to an already-seeded
// reseller/inbound pair, so multi-client cascade behavior can be tested.
func addClientToReseller(t *testing.T, reseller *model.User, inbound *model.Inbound, email string) {
	t.Helper()
	db := database.GetDB()

	client := model.Client{
		Email:      email,
		ID:         "22222222-2222-2222-2222-222222222222",
		SubID:      email,
		Enable:     true,
		TotalGB:    0,
		ExpiryTime: 0,
	}

	var settings struct {
		Clients []model.Client `json:"clients"`
	}
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		t.Fatalf("unmarshal inbound settings: %v", err)
	}
	settings.Clients = append(settings.Clients, client)
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	if err := db.Model(inbound).Update("settings", string(settingsJSON)).Error; err != nil {
		t.Fatalf("update inbound settings: %v", err)
	}

	rec := &model.ClientRecord{
		Email:   email,
		UUID:    client.ID,
		SubID:   email,
		Enable:  true,
		OwnerId: reseller.Id,
	}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("create client record: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: inbound.Id}).Error; err != nil {
		t.Fatalf("create client-inbound link: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: email, Enable: true, InboundId: inbound.Id}).Error; err != nil {
		t.Fatalf("create client traffic: %v", err)
	}
}

func clientRecordEnable(t *testing.T, email string) bool {
	t.Helper()
	rec := &model.ClientRecord{}
	if err := database.GetDB().Where("email = ?", email).First(rec).Error; err != nil {
		t.Fatalf("load client record %q: %v", email, err)
	}
	return rec.Enable
}

// inboundSettingsClientEnable reads a client's "enable" flag straight out of
// the inbound's Settings JSON — the field xray-core and node sync actually
// read — to prove a re-enable was pushed live, not just flipped in the
// ClientRecord row.
func inboundSettingsClientEnable(t *testing.T, inboundId int, email string) bool {
	t.Helper()
	ib := &model.Inbound{}
	if err := database.GetDB().First(ib, inboundId).Error; err != nil {
		t.Fatalf("load inbound %d: %v", inboundId, err)
	}
	var settings struct {
		Clients []model.Client `json:"clients"`
	}
	if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
		t.Fatalf("unmarshal inbound settings: %v", err)
	}
	for _, c := range settings.Clients {
		if c.Email == email {
			return c.Enable
		}
	}
	t.Fatalf("client %q not found in inbound %d settings", email, inboundId)
	return false
}

// TestToggleEnableReseller_CascadesToClients is the regression test for the
// reported bug: disabling a reseller in the panel locked them out of their
// own login, but their clients kept working (and even accepting NEW
// connections) because nothing pushed the disabled state to the clients or
// to the live Xray config. ToggleEnableReseller must cascade through
// ClientService.BulkSetEnable, the same mechanism the ordinary per-client
// enable/disable toggle uses.
func TestToggleEnableReseller_CascadesToClients(t *testing.T) {
	setupBotTestDB(t)
	email := "cascade@x"
	reseller, _ := seedResellerWithClient(t, email)

	svc := &ResellerService{}

	enabled, _, err := svc.ToggleEnableReseller(reseller.Id)
	if err != nil {
		t.Fatalf("ToggleEnableReseller (disable): %v", err)
	}
	if enabled {
		t.Fatalf("ToggleEnableReseller (disable) returned enabled=true, want false")
	}
	if got := clientRecordEnable(t, email); got {
		t.Fatalf("client %q still enabled after disabling its reseller, want disabled", email)
	}

	enabled, _, err = svc.ToggleEnableReseller(reseller.Id)
	if err != nil {
		t.Fatalf("ToggleEnableReseller (re-enable): %v", err)
	}
	if !enabled {
		t.Fatalf("ToggleEnableReseller (re-enable) returned enabled=false, want true")
	}
	if got := clientRecordEnable(t, email); !got {
		t.Fatalf("client %q still disabled after re-enabling its reseller, want enabled", email)
	}
}

// TestToggleEnableReseller_DoesNotTouchOtherResellersClients guards against
// an over-broad cascade — a common way to get this fix wrong is to filter by
// email substring or forget the owner_id scope.
func TestToggleEnableReseller_DoesNotTouchOtherResellersClients(t *testing.T) {
	setupBotTestDB(t)
	victimEmail := "victim@x"
	targetEmail := "target@x"
	_, _ = seedResellerWithClient(t, victimEmail)
	targetReseller, _ := seedResellerWithClient(t, targetEmail)

	svc := &ResellerService{}
	if _, _, err := svc.ToggleEnableReseller(targetReseller.Id); err != nil {
		t.Fatalf("ToggleEnableReseller: %v", err)
	}

	if got := clientRecordEnable(t, targetEmail); got {
		t.Fatalf("target reseller's client still enabled, want disabled")
	}
	if got := clientRecordEnable(t, victimEmail); !got {
		t.Fatalf("unrelated reseller's client was disabled too, want untouched (still enabled)")
	}
}

// TestToggleEnableReseller_ReEnableSkipsManuallyDisabledClients is the
// regression test for the reported bug: re-enabling a reseller must only
// restore the clients the disable cascade itself turned off, not a client
// the reseller had already disabled manually before the reseller-level
// disable — otherwise re-enabling silently undoes the reseller's own choice.
func TestToggleEnableReseller_ReEnableSkipsManuallyDisabledClients(t *testing.T) {
	setupBotTestDB(t)
	cascadeEmail := "cascade2@x"
	manualEmail := "manual2@x"
	reseller, inbound := seedResellerWithClient(t, cascadeEmail)
	addClientToReseller(t, reseller, inbound, manualEmail)

	// Disable manualEmail the same way the ordinary per-client toggle does
	// (through BulkSetEnable, which updates the inbound Settings JSON, the
	// client_traffics row and the ClientRecord row together) — a raw
	// ClientRecord-only update would be inconsistent with the inbound's
	// Settings JSON and get overwritten by the next resync.
	if _, _, err := (&service.ClientService{}).BulkSetEnable(&service.InboundService{}, []string{manualEmail}, false); err != nil {
		t.Fatalf("manually disable client: %v", err)
	}

	svc := &ResellerService{}
	if _, _, err := svc.ToggleEnableReseller(reseller.Id); err != nil {
		t.Fatalf("ToggleEnableReseller (disable): %v", err)
	}
	if got := clientRecordEnable(t, cascadeEmail); got {
		t.Fatalf("cascade-target client still enabled after disabling its reseller, want disabled")
	}

	enabled, _, err := svc.ToggleEnableReseller(reseller.Id)
	if err != nil {
		t.Fatalf("ToggleEnableReseller (re-enable): %v", err)
	}
	if !enabled {
		t.Fatalf("ToggleEnableReseller (re-enable) returned enabled=false, want true")
	}
	if got := clientRecordEnable(t, cascadeEmail); !got {
		t.Fatalf("cascade-disabled client %q still disabled after re-enabling its reseller, want enabled", cascadeEmail)
	}
	if got := clientRecordEnable(t, manualEmail); got {
		t.Fatalf("manually-disabled client %q was re-enabled by reseller re-enable, want still disabled", manualEmail)
	}
}

// TestResetResellerUsage_OnlyReEnablesLockedClientsLive is the regression
// test for the "recharge" path: it previously force-enabled every client of
// the reseller (undoing manual disables) via a raw DB update that never
// touched the inbound's Settings JSON, so a quota-locked client stayed
// removed from the live Xray config even after the reset reported it
// "re-enabled". It must now only restore quota-locked clients, and it must
// do so through the same live-push path as every other enable cascade.
func TestResetResellerUsage_OnlyReEnablesLockedClientsLive(t *testing.T) {
	setupBotTestDB(t)
	lockedEmail := "locked3@x"
	manualEmail := "manual3@x"
	reseller, inbound := seedResellerWithClient(t, lockedEmail)
	addClientToReseller(t, reseller, inbound, manualEmail)

	db := database.GetDB()
	if err := db.Model(&model.User{}).Where("id = ?", reseller.Id).
		Updates(map[string]any{"usage_limit": 100, "usage_up": 100, "usage_down": 0}).Error; err != nil {
		t.Fatalf("set reseller over quota: %v", err)
	}

	// lockedEmail gets auto-disabled by the over-quota job (Locked=true,
	// pushed live through BulkSetEnable so its inbound JSON entry flips too).
	if _, _, err := (&service.ClientService{}).BulkSetEnable(&service.InboundService{}, []string{lockedEmail}, false); err != nil {
		t.Fatalf("simulate quota auto-disable: %v", err)
	}
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", lockedEmail).Update("locked", true).Error; err != nil {
		t.Fatalf("mark client locked: %v", err)
	}
	// manualEmail was disabled by the reseller themselves, unrelated to quota.
	if _, _, err := (&service.ClientService{}).BulkSetEnable(&service.InboundService{}, []string{manualEmail}, false); err != nil {
		t.Fatalf("manually disable client: %v", err)
	}

	svc := &ResellerService{}
	affected, err := svc.ResetResellerUsage(reseller.Id)
	if err != nil {
		t.Fatalf("ResetResellerUsage: %v", err)
	}
	if affected != 1 {
		t.Fatalf("ResetResellerUsage reported %d re-enabled, want 1 (only the quota-locked client)", affected)
	}

	if got := clientRecordEnable(t, lockedEmail); !got {
		t.Fatalf("quota-locked client %q still disabled after usage reset, want enabled", lockedEmail)
	}
	if got := inboundSettingsClientEnable(t, inbound.Id, lockedEmail); !got {
		t.Fatalf("quota-locked client %q not re-enabled in the live inbound settings, want enabled", lockedEmail)
	}
	if got := clientRecordEnable(t, manualEmail); got {
		t.Fatalf("manually-disabled client %q was re-enabled by usage reset, want still disabled", manualEmail)
	}
	if got := inboundSettingsClientEnable(t, inbound.Id, manualEmail); got {
		t.Fatalf("manually-disabled client %q was re-enabled in the live inbound settings, want still disabled", manualEmail)
	}
}

// TestResellerPassword_StoredCreatedUpdatedAndRead covers the owner-visible
// plaintext: it is saved on create, replaced only when a non-blank password is
// supplied on update, and read back by GetResellerPassword.
func TestResellerPassword_StoredCreatedUpdatedAndRead(t *testing.T) {
	setupBotTestDB(t)
	svc := &ResellerService{}

	created, err := svc.CreateReseller("pw-reseller", "initialPass1", 0, "all", nil, 1)
	if err != nil {
		t.Fatalf("CreateReseller: %v", err)
	}

	if got, err := svc.GetResellerPassword(created.Id); err != nil || got != "initialPass1" {
		t.Fatalf("GetResellerPassword after create = %q, %v; want %q, nil", got, err, "initialPass1")
	}

	if err := svc.UpdateReseller(created.Id, "pw-reseller", "", 0, "all", nil, 1); err != nil {
		t.Fatalf("UpdateReseller (blank password): %v", err)
	}
	if got, _ := svc.GetResellerPassword(created.Id); got != "initialPass1" {
		t.Fatalf("stored password after blank update = %q, want %q (unchanged)", got, "initialPass1")
	}

	if err := svc.UpdateReseller(created.Id, "pw-reseller", "changedPass2", 0, "all", nil, 1); err != nil {
		t.Fatalf("UpdateReseller (new password): %v", err)
	}
	if got, _ := svc.GetResellerPassword(created.Id); got != "changedPass2" {
		t.Fatalf("stored password after update = %q, want %q", got, "changedPass2")
	}

	var reloaded model.User
	if err := database.GetDB().First(&reloaded, created.Id).Error; err != nil {
		t.Fatalf("reload reseller: %v", err)
	}
	if !crypto.CheckPasswordHash(reloaded.Password, "changedPass2") {
		t.Fatalf("password hash does not verify against the updated password")
	}
}

func TestGetResellerPassword_RejectsNonReseller(t *testing.T) {
	setupBotTestDB(t)
	admin := &model.User{Username: "an-admin", Password: "x", Role: "admin"}
	if err := database.GetDB().Create(admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if _, err := (&ResellerService{}).GetResellerPassword(admin.Id); err == nil {
		t.Fatalf("GetResellerPassword returned nil error for a non-reseller user, want error")
	}
}

func reloadUserEpoch(t *testing.T, id int) int64 {
	t.Helper()
	var u model.User
	if err := database.GetDB().First(&u, id).Error; err != nil {
		t.Fatalf("reload user %d: %v", id, err)
	}
	return u.LoginEpoch
}

// TestResetResellerUsage_ClearsQuotaLoginLock covers the account half of the
// over-quota lockdown: a reseller auto-locked out of the panel for going over
// quota regains login access on a usage reset, while a reseller the owner
// disabled by hand (LoginEpoch -1) stays locked out. -1000000 mirrors
// service.ResellerQuotaLockEpoch.
func TestResetResellerUsage_ClearsQuotaLoginLock(t *testing.T) {
	setupBotTestDB(t)
	db := database.GetDB()
	svc := &ResellerService{}

	quotaLocked, _ := seedResellerWithClient(t, "qlock@x")
	if err := db.Model(&model.User{}).Where("id = ?", quotaLocked.Id).
		Updates(map[string]any{"usage_limit": 100, "usage_up": 100, "login_epoch": -1000000}).Error; err != nil {
		t.Fatalf("set reseller quota-locked: %v", err)
	}
	if _, err := svc.ResetResellerUsage(quotaLocked.Id); err != nil {
		t.Fatalf("ResetResellerUsage (quota-locked): %v", err)
	}
	if got := reloadUserEpoch(t, quotaLocked.Id); got != 0 {
		t.Fatalf("quota-locked reseller LoginEpoch = %d after reset, want 0 (re-enabled)", got)
	}

	manual, _ := seedResellerWithClient(t, "mlock@x")
	if err := db.Model(&model.User{}).Where("id = ?", manual.Id).
		Updates(map[string]any{"usage_limit": 100, "usage_up": 100, "login_epoch": -1}).Error; err != nil {
		t.Fatalf("set reseller manually disabled: %v", err)
	}
	if _, err := svc.ResetResellerUsage(manual.Id); err != nil {
		t.Fatalf("ResetResellerUsage (manual): %v", err)
	}
	if got := reloadUserEpoch(t, manual.Id); got != -1 {
		t.Fatalf("manually-disabled reseller LoginEpoch = %d after reset, want -1 (still locked)", got)
	}
}

// TestToggleEnableReseller_OverQuotaUsesSentinelAndRefusesEnable: disabling an
// over-quota reseller stamps the quota sentinel (so a later usage reset clears
// it), and re-enabling one while still over quota is refused rather than
// producing a few seconds of access before the traffic job re-locks it.
func TestToggleEnableReseller_OverQuotaUsesSentinelAndRefusesEnable(t *testing.T) {
	setupBotTestDB(t)
	db := database.GetDB()
	svc := &ResellerService{}

	reseller, _ := seedResellerWithClient(t, "oq-toggle@x")
	if err := db.Model(&model.User{}).Where("id = ?", reseller.Id).
		Updates(map[string]any{"usage_limit": 100, "usage_up": 100}).Error; err != nil {
		t.Fatalf("set over quota: %v", err)
	}

	enabled, _, err := svc.ToggleEnableReseller(reseller.Id)
	if err != nil {
		t.Fatalf("ToggleEnableReseller (disable): %v", err)
	}
	if enabled {
		t.Fatalf("reseller still enabled after disable toggle")
	}
	if got := reloadUserEpoch(t, reseller.Id); got != service.ResellerQuotaLockEpoch {
		t.Fatalf("LoginEpoch after over-quota disable = %d, want %d (quota sentinel)", got, service.ResellerQuotaLockEpoch)
	}

	if _, _, err := svc.ToggleEnableReseller(reseller.Id); err == nil {
		t.Fatalf("ToggleEnableReseller (enable while over quota) returned nil error, want refusal")
	}
	if got := reloadUserEpoch(t, reseller.Id); got != service.ResellerQuotaLockEpoch {
		t.Fatalf("LoginEpoch changed on refused enable = %d, want %d", got, service.ResellerQuotaLockEpoch)
	}

	if _, err := svc.ResetResellerUsage(reseller.Id); err != nil {
		t.Fatalf("ResetResellerUsage: %v", err)
	}
	if got := reloadUserEpoch(t, reseller.Id); got != 0 {
		t.Fatalf("LoginEpoch after reset = %d, want 0 (re-enabled)", got)
	}
}

// TestToggleEnableReseller_UnderQuotaDisableStaysManual guards the sentinel from
// over-firing: a reseller within quota still gets a plain -1 on disable, which a
// usage reset must not clear.
func TestToggleEnableReseller_UnderQuotaDisableStaysManual(t *testing.T) {
	setupBotTestDB(t)
	svc := &ResellerService{}

	reseller, _ := seedResellerWithClient(t, "uq-toggle@x")
	if _, _, err := svc.ToggleEnableReseller(reseller.Id); err != nil {
		t.Fatalf("ToggleEnableReseller (disable): %v", err)
	}
	if got := reloadUserEpoch(t, reseller.Id); got != -1 {
		t.Fatalf("LoginEpoch after under-quota disable = %d, want -1 (manual)", got)
	}
}
