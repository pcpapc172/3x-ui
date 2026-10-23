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

func TestGlobalTrafficDoesNotBillRepeatedMasterSnapshots(t *testing.T) {
	db := initTrafficTestDB(t)
	owner := &model.User{Username: "global-owner", Role: "reseller", UsageLimit: 3 * 1024 * 1024 * 1024 * 1024}
	if err := db.Create(owner).Error; err != nil {
		t.Fatal(err)
	}
	seedMultiplierClient(t, db, 1, "master-client", owner.Id)
	svc := &InboundService{}
	for range 3 {
		if err := svc.AcceptGlobalTraffic("master", []*xray.ClientTraffic{{Email: "master-client", Up: 100, Down: 200}}); err != nil {
			t.Fatal(err)
		}
	}
	var got model.User
	if err := db.First(&got, owner.Id).Error; err != nil {
		t.Fatal(err)
	}
	if got.UsageUp != 0 || got.UsageDown != 0 {
		t.Fatalf("master display snapshot billed again: up=%d down=%d, want 0/0 (local traffic already accounted)", got.UsageUp, got.UsageDown)
	}
}

func TestAddTrafficMultiplierCannotOverflowQuotaCounters(t *testing.T) {
	db := initTrafficTestDB(t)
	inbound := &model.Inbound{UserId: 1, Tag: "overflow-mult", Enable: true, Port: 44032, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatal(err)
	}
	owner := &model.User{Username: "overflow-owner", Role: "reseller", Multiplier: 4}
	if err := db.Create(owner).Error; err != nil {
		t.Fatal(err)
	}
	seedMultiplierClient(t, db, inbound.Id, "overflow-client", owner.Id)
	if _, _, err := (&InboundService{}).AddTraffic(nil, []*xray.ClientTraffic{{Email: "overflow-client", Up: 4_000_000_000_000_000_000}}); err != nil {
		t.Fatal(err)
	}
	got := readTraffic(t, db, "overflow-client")
	if got.Up != 9_000_000_000_000_000_000 {
		t.Fatalf("multiplied up=%d, want saturated 9000000000000000000", got.Up)
	}
	var user model.User
	if err := db.First(&user, owner.Id).Error; err != nil {
		t.Fatal(err)
	}
	if user.UsageUp != got.Up {
		t.Fatalf("owner up=%d, want %d", user.UsageUp, got.Up)
	}
}

func TestNodeMultiplierBillsClientAndOwnerOnlyForNewTraffic(t *testing.T) {
	db := initTrafficTestDB(t)
	nodeID, _ := setupNodeRuntime(t)
	owner := &model.User{Username: "node-owner", Role: "reseller", Multiplier: 2}
	if err := db.Create(owner).Error; err != nil {
		t.Fatal(err)
	}
	client := model.Client{Email: "node-mult-client", Enable: true, OwnerId: owner.Id}
	ib := nodeInbound(t, nodeID, 44033, []model.Client{client})
	if err := db.Create(&model.InboundMultiplier{InboundId: ib.Id, Rate: 3}).Error; err != nil {
		t.Fatal(err)
	}
	seedClientTraffic(t, ib.Id, client.Email, true)
	svc := &InboundService{}
	syncNode(t, svc, nodeID, ib.Tag, xray.ClientTraffic{Email: client.Email, Enable: true})
	for range 2 {
		syncNode(t, svc, nodeID, ib.Tag, xray.ClientTraffic{Email: client.Email, Enable: true, Up: 100, Down: 50})
	}
	got := readTraffic(t, db, client.Email)
	if got.Up != 600 || got.Down != 300 {
		t.Fatalf("node client up/down=%d/%d, want 600/300 without replay billing", got.Up, got.Down)
	}
	var user model.User
	if err := db.First(&user, owner.Id).Error; err != nil {
		t.Fatal(err)
	}
	if user.UsageUp != 600 || user.UsageDown != 300 {
		t.Fatalf("node owner up/down=%d/%d, want 600/300", user.UsageUp, user.UsageDown)
	}
}

func TestAddTrafficDoesNotCompoundReusedDelta(t *testing.T) {
	db := initTrafficTestDB(t)
	ib := &model.Inbound{UserId: 1, Tag: "reused-mult", Port: 44034, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.InboundMultiplier{InboundId: ib.Id, Rate: 2}).Error; err != nil {
		t.Fatal(err)
	}
	seedMultiplierClient(t, db, ib.Id, "reused-client", 0)
	delta := []*xray.ClientTraffic{{Email: "reused-client", Up: 100}}
	for range 2 {
		if _, _, err := (&InboundService{}).AddTraffic(nil, delta); err != nil {
			t.Fatal(err)
		}
	}
	got := readTraffic(t, db, "reused-client")
	if got.Up != 400 {
		t.Fatalf("two 100-byte ticks at 2x billed %d, want 400", got.Up)
	}
}
