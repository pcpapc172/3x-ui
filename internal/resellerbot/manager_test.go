package resellerbot

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestDisabledSalesBotsNeverStartSavedEnabledConfigs(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	cfg := &model.BotConfig{OwnerId: 1, Enabled: true, Token: "invalid-saved-token"}
	if err := database.GetDB().Create(cfg).Error; err != nil {
		t.Fatal(err)
	}
	m := &Manager{instances: make(map[int]*botInstance)}
	if err := m.Ensure(cfg.OwnerId); err != nil {
		t.Fatalf("default-disabled sales bot attempted startup: %v", err)
	}
	var saved model.BotConfig
	if err := database.GetDB().First(&saved, cfg.Id).Error; err != nil {
		t.Fatal(err)
	}
	if !saved.Enabled || saved.Token != cfg.Token {
		t.Fatalf("global disable must preserve owner configuration: enabled=%v token preserved=%v", saved.Enabled, saved.Token == cfg.Token)
	}
}
