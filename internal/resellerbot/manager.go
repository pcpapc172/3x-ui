// Package resellerbot runs one independent long-polling Telegram bot per
// panel user (reseller or admin) who has registered their own bot token,
// selling plans to buyers with manual payment review. It mirrors
// internal/mtproto's map+mutex registry pattern for managing N concurrent
// per-owner instances, but each instance here is a goroutine, not an OS
// process.
package resellerbot

import (
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// Manager owns the set of running bot instances keyed by owner id.
type Manager struct {
	mu        sync.Mutex
	instances map[int]*botInstance
}

var (
	managerOnce sync.Once
	manager     *Manager
)

// GetManager returns the process-wide reseller-bot manager singleton.
func GetManager() *Manager {
	managerOnce.Do(func() {
		manager = &Manager{instances: map[int]*botInstance{}}
	})
	return manager
}

// Ensure starts the bot for an owner, or restarts it if the token changed,
// or stops it if the config is now disabled/tokenless. A no-op when the
// desired state already matches what's running.
func (m *Manager) Ensure(ownerId int) error {
	cfg := &model.BotConfig{}
	err := database.GetDB().Where("owner_id = ?", ownerId).First(cfg).Error
	if err != nil {
		m.Stop(ownerId)
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	enabled, err := (&service.SettingService{}).GetResellerBotEnabled()
	if err != nil || !enabled || !cfg.Enabled || cfg.Token == "" {
		if cur, ok := m.instances[ownerId]; ok {
			cur.stop()
			delete(m.instances, ownerId)
		}
		return nil
	}

	if cur, ok := m.instances[ownerId]; ok {
		if cur.token == cfg.Token {
			return nil
		}
		cur.stop()
		delete(m.instances, ownerId)
	}

	inst, err := startBotInstance(ownerId, cfg.Token)
	if err != nil {
		logger.Warningf("resellerbot: failed to start bot for owner %d: %v", ownerId, err)
		return err
	}
	m.instances[ownerId] = inst
	return nil
}

// Stop stops and forgets the bot instance for an owner id, if running.
func (m *Manager) Stop(ownerId int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cur, ok := m.instances[ownerId]; ok {
		cur.stop()
		delete(m.instances, ownerId)
	}
}

// Reconcile drives the running set toward every enabled BotConfig row —
// starting missing instances and stopping any that no longer have an
// enabled config. Used at boot and by a periodic job to self-heal.
func (m *Manager) Reconcile() {
	enabled, err := (&service.SettingService{}).GetResellerBotEnabled()
	if err != nil || !enabled {
		m.StopAll()
		return
	}
	var configs []model.BotConfig
	if err := database.GetDB().Where("enabled = ? AND token != ''", true).Find(&configs).Error; err != nil {
		logger.Warning("resellerbot: reconcile failed to load configs:", err)
		return
	}
	want := make(map[int]string, len(configs))
	for _, c := range configs {
		want[c.OwnerId] = c.Token
	}

	m.mu.Lock()
	for ownerId, cur := range m.instances {
		if want[ownerId] != cur.token {
			cur.stop()
			delete(m.instances, ownerId)
		}
	}
	m.mu.Unlock()

	for ownerId := range want {
		if err := m.Ensure(ownerId); err != nil {
			logger.Warningf("resellerbot: reconcile failed for owner %d: %v", ownerId, err)
		}
	}
}

// StopAll stops every running bot instance. Called on panel shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for ownerId, cur := range m.instances {
		cur.stop()
		delete(m.instances, ownerId)
	}
}

func (m *Manager) instanceFor(ownerId int) (*botInstance, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inst, ok := m.instances[ownerId]
	return inst, ok
}
