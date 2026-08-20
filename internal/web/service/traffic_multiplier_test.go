package service

import (
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// seedMultiplierClient creates a client owned by ownerId (0 = unowned/admin),
// attached to inboundId, with a pre-existing zeroed traffic row — the shape
// addClientTraffic/applyTrafficMultipliers expect to find.
func seedMultiplierClient(t *testing.T, db *gorm.DB, inboundId int, email string, ownerId int) {
	t.Helper()
	rec := &model.ClientRecord{Email: email, Enable: true, OwnerId: ownerId}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("seed client record %q: %v", email, err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: inboundId}).Error; err != nil {
		t.Fatalf("seed client-inbound link %q: %v", email, err)
	}
	if err := db.Create(&xray.ClientTraffic{InboundId: inboundId, Email: email, Enable: true}).Error; err != nil {
		t.Fatalf("seed traffic row %q: %v", email, err)
	}
}

// TestAddTraffic_MultiplierAppliesToClientAndReseller is the regression test
// for the reported bug: an inbound or reseller multiplier was applied to the
// reseller's aggregate usage (User.UsageUp/UsageDown) but never to the
// client's own traffic counter, so the client's own displayed usage/quota
// consumption stayed raw. The multiplier must now apply consistently:
//   - an inbound multiplier affects every client on it, owned or not
//   - a reseller multiplier additionally affects only that reseller's clients
func TestAddTraffic_MultiplierAppliesToClientAndReseller(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	inbound := &model.Inbound{UserId: 1, Tag: "mult-in", Enable: true, Port: 44001, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if err := db.Create(&model.InboundMultiplier{InboundId: inbound.Id, Rate: 3}).Error; err != nil {
		t.Fatalf("seed inbound multiplier: %v", err)
	}

	reseller := &model.User{Username: "reseller-mult", Password: "x", Role: "reseller", Multiplier: 2}
	if err := db.Create(reseller).Error; err != nil {
		t.Fatalf("seed reseller: %v", err)
	}

	seedMultiplierClient(t, db, inbound.Id, "resold@x", reseller.Id)
	seedMultiplierClient(t, db, inbound.Id, "admin-owned@x", 0)

	if _, _, err := svc.AddTraffic(nil, []*xray.ClientTraffic{
		{Email: "resold@x", Up: 100, Down: 50},
		{Email: "admin-owned@x", Up: 100, Down: 50},
	}); err != nil {
		t.Fatalf("AddTraffic: %v", err)
	}

	// Reseller's client: inbound rate (3) * reseller rate (2) = 6x.
	resold := readTraffic(t, db, "resold@x")
	assertUpDown(t, resold, 600, 300, "reseller-owned client's own traffic")

	// Unowned client on the same multiplied inbound: inbound rate only, 3x.
	adminOwned := readTraffic(t, db, "admin-owned@x")
	assertUpDown(t, adminOwned, 300, 150, "unowned client's own traffic")

	// Reseller aggregate must match the reseller-owned client's multiplied
	// delta (600/300), not the raw delta (100/50).
	var reloadedReseller model.User
	if err := db.First(&reloadedReseller, reseller.Id).Error; err != nil {
		t.Fatalf("reload reseller: %v", err)
	}
	if reloadedReseller.UsageUp != 600 || reloadedReseller.UsageDown != 300 {
		t.Fatalf("reseller aggregate = up=%d down=%d, want 600/300 (matching the client's own multiplied usage)",
			reloadedReseller.UsageUp, reloadedReseller.UsageDown)
	}
}
