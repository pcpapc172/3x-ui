package panel

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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

func clientRecordEnable(t *testing.T, email string) bool {
	t.Helper()
	rec := &model.ClientRecord{}
	if err := database.GetDB().Where("email = ?", email).First(rec).Error; err != nil {
		t.Fatalf("load client record %q: %v", email, err)
	}
	return rec.Enable
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
