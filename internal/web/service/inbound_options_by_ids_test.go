package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestGetInboundOptionsByIds_Enable reproduces a live regression: a reseller
// with AllowedInboundsMode "select" is served options through this function
// (internal/web/controller/inbound.go's getInboundOptions), and the frontend
// client-creation picker only shows an inbound when Enable is true (or it's
// already attached). This function used to never populate Enable, so every
// enabled inbound was silently hidden from every such reseller's "Add Client"
// dialog.
func TestGetInboundOptionsByIds_Enable(t *testing.T) {
	setupConflictDB(t)

	enabled := &model.Inbound{
		UserId:         1,
		Tag:            "in-443-tcp",
		Enable:         true,
		Listen:         "0.0.0.0",
		Port:           443,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
		Settings:       `{"clients":[]}`,
	}
	disabled := &model.Inbound{
		UserId:         1,
		Tag:            "in-444-tcp",
		Enable:         false,
		Listen:         "0.0.0.0",
		Port:           444,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
		Settings:       `{"clients":[]}`,
	}
	if err := database.GetDB().Create(enabled).Error; err != nil {
		t.Fatalf("create enabled inbound: %v", err)
	}
	if err := database.GetDB().Create(disabled).Error; err != nil {
		t.Fatalf("create disabled inbound: %v", err)
	}

	svc := &InboundService{}
	options, err := svc.GetInboundOptionsByIds([]int{enabled.Id, disabled.Id})
	if err != nil {
		t.Fatalf("GetInboundOptionsByIds: %v", err)
	}

	byID := make(map[int]InboundOption, len(options))
	for _, o := range options {
		byID[o.Id] = o
	}

	got, ok := byID[enabled.Id]
	if !ok {
		t.Fatalf("enabled inbound %d missing from options", enabled.Id)
	}
	if !got.Enable {
		t.Fatalf("enabled inbound Enable = false, want true — this is the reported bug: a reseller's client-creation picker filters on Enable, so this hides the inbound entirely")
	}

	gotDisabled, ok := byID[disabled.Id]
	if !ok {
		t.Fatalf("disabled inbound %d missing from options", disabled.Id)
	}
	if gotDisabled.Enable {
		t.Fatalf("disabled inbound Enable = true, want false")
	}
}
