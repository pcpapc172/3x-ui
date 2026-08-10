package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestBulkAdjust_RenewalPreservesUsage is the regression test for the "renew
// must never reset usage" requirement (the reseller bot's renewal flow calls
// BulkAdjust exactly this way — addDays/addBytes with an empty flow — rather
// than provisioning a fresh client or resetting traffic).
func TestBulkAdjust_RenewalPreservesUsage(t *testing.T) {
	const gbBytes = 1024 * 1024 * 1024

	cases := []struct {
		name           string
		startExpiry    int64
		durationDays   int
		wantExpiryDiff int64 // want newExpiry - startExpiry
	}{
		{
			name:           "delayed-start client (not yet connected)",
			startExpiry:    -(30 * 24 * 60 * 60 * 1000), // 30 days, not started
			durationDays:   30,
			wantExpiryDiff: -(30 * 24 * 60 * 60 * 1000),
		},
		{
			name:           "already-active client with a fixed future expiry",
			startExpiry:    2000000000000, // arbitrary fixed future timestamp (ms)
			durationDays:   30,
			wantExpiryDiff: 30 * 24 * 60 * 60 * 1000,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupBulkDB(t)
			svc := &ClientService{}
			inboundSvc := &InboundService{}

			email := "renew@x"
			c := model.Client{
				Email:      email,
				ID:         "11111111-1111-1111-1111-111111111111",
				SubID:      email,
				TotalGB:    5 * gbBytes,
				ExpiryTime: tc.startExpiry,
				Enable:     true,
			}
			ib := mkInbound(t, 53200+i, model.VLESS, clientsSettings(t, []model.Client{c}))
			if err := svc.SyncInbound(nil, ib.Id, []model.Client{c}); err != nil {
				t.Fatalf("seed linkage: %v", err)
			}
			mkTraffic(t, ib.Id, email, 1_000_000_000, 2_000_000_000, c.TotalGB, c.ExpiryTime, true)

			addBytes := int64(10) * gbBytes // renewal plan: +10GB
			result, _, err := svc.BulkAdjust(inboundSvc, []string{email}, tc.durationDays, addBytes, "")
			if err != nil {
				t.Fatalf("BulkAdjust: %v", err)
			}
			if result.Adjusted != 1 {
				t.Fatalf("Adjusted = %d, want 1 (skipped=%v)", result.Adjusted, result.Skipped)
			}

			var tr xray.ClientTraffic
			if err := database.GetDB().Where("email = ?", email).First(&tr).Error; err != nil {
				t.Fatalf("load traffic: %v", err)
			}
			if tr.Up != 1_000_000_000 || tr.Down != 2_000_000_000 {
				t.Fatalf("usage counters changed by renewal: up=%d down=%d, want 1000000000/2000000000", tr.Up, tr.Down)
			}
			if wantTotal := c.TotalGB + addBytes; tr.Total != wantTotal {
				t.Fatalf("Total = %d, want %d (existing quota + plan traffic, not replaced)", tr.Total, wantTotal)
			}
			if wantExpiry := tc.startExpiry + tc.wantExpiryDiff; tr.ExpiryTime != wantExpiry {
				t.Fatalf("ExpiryTime = %d, want %d (extended, not reset)", tr.ExpiryTime, wantExpiry)
			}
		})
	}
}
