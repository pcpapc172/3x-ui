package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestRegenerateCredentialsByEmail_PreservesUsageAndQuota is the regression
// test for the "revoke link must never reset traffic" requirement: a client
// with real usage/quota gets new credentials, and every counter must survive
// byte-for-byte.
func TestRegenerateCredentialsByEmail_PreservesUsageAndQuota(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	email := "revoke-me@x"
	oldID := "11111111-1111-1111-1111-111111111111"
	oldSubID := "old-sub-id"
	c := model.Client{
		Email:      email,
		ID:         oldID,
		SubID:      oldSubID,
		TotalGB:    5 * 1024 * 1024 * 1024,
		ExpiryTime: 1893456000000, // fixed future timestamp
		Enable:     true,
	}
	ib := mkInbound(t, 53101, model.VLESS, clientsSettings(t, []model.Client{c}))
	if err := svc.SyncInbound(nil, ib.Id, []model.Client{c}); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	mkTraffic(t, ib.Id, email, 1234, 5678, c.TotalGB, c.ExpiryTime, true)

	newID, newSubID, err := svc.RegenerateCredentialsByEmail(inboundSvc, email)
	if err != nil {
		t.Fatalf("RegenerateCredentialsByEmail: %v", err)
	}
	if newID == oldID || newSubID == oldSubID {
		t.Fatalf("credentials not rotated: newID=%q newSubID=%q", newID, newSubID)
	}

	rec, err := svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if rec.UUID != newID || rec.SubID != newSubID {
		t.Fatalf("record not updated: got uuid=%q subId=%q, want %q/%q", rec.UUID, rec.SubID, newID, newSubID)
	}
	if rec.TotalGB != c.TotalGB {
		t.Fatalf("TotalGB changed by revoke: got %d, want %d", rec.TotalGB, c.TotalGB)
	}
	if rec.ExpiryTime != c.ExpiryTime {
		t.Fatalf("ExpiryTime changed by revoke: got %d, want %d", rec.ExpiryTime, c.ExpiryTime)
	}
	if !rec.Enable {
		t.Fatalf("Enable changed by revoke: got false, want true")
	}

	var tr xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", email).First(&tr).Error; err != nil {
		t.Fatalf("load traffic: %v", err)
	}
	if tr.Up != 1234 || tr.Down != 5678 {
		t.Fatalf("usage counters changed by revoke: up=%d down=%d, want 1234/5678", tr.Up, tr.Down)
	}
}
