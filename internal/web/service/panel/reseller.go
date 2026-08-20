package panel

import (
	"encoding/json"
	"errors"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"gorm.io/gorm"
)

// ResellerInfo is the admin-facing projection of a reseller account.
type ResellerInfo struct {
	Id                  int     `json:"id"`
	Username            string  `json:"username"`
	Role                string  `json:"role"`
	UsageLimit          int64   `json:"usageLimit"`
	UsageUp             int64   `json:"usageUp"`
	UsageDown           int64   `json:"usageDown"`
	ClientCount         int64   `json:"clientCount"`
	Enabled             bool    `json:"enabled"`
	AllowedInboundsMode string  `json:"allowedInboundsMode"`
	AllowedInboundIds   []int   `json:"allowedInboundIds"`
	Multiplier          float64 `json:"multiplier"`
}

// ResellerService provides CRUD operations for reseller accounts.
type ResellerService struct{}

func (s *ResellerService) ListResellers() ([]ResellerInfo, error) {
	db := database.GetDB()
	var users []model.User
	if err := db.Where("role = ?", "reseller").Find(&users).Error; err != nil {
		return nil, err
	}
	result := make([]ResellerInfo, 0, len(users))
	for _, u := range users {
		var count int64
		_ = db.Model(&model.ClientRecord{}).Where("owner_id = ?", u.Id).Count(&count).Error
		result = append(result, ResellerInfo{
			Id:                  u.Id,
			Username:            u.Username,
			Role:                u.Role,
			UsageLimit:          u.UsageLimit,
			UsageUp:             u.UsageUp,
			UsageDown:           u.UsageDown,
			ClientCount:         count,
			Enabled:             u.LoginEpoch >= 0,
			AllowedInboundsMode: u.AllowedInboundsMode,
			AllowedInboundIds:   u.AllowedInboundIds,
			Multiplier:          u.Multiplier,
		})
	}
	return result, nil
}

func (s *ResellerService) CreateReseller(username, password string, usageLimit int64, allowedInboundsMode string, allowedInboundIds []int, multiplier float64) (*model.User, error) {
	if username == "" {
		return nil, errors.New("username is required")
	}
	if password == "" {
		return nil, errors.New("password is required")
	}
	if allowedInboundsMode == "" {
		allowedInboundsMode = "all"
	}
	if multiplier < 1 {
		multiplier = 1
	}
	db := database.GetDB()
	var exists int64
	db.Model(&model.User{}).Where("username = ?", username).Count(&exists)
	if exists > 0 {
		return nil, errors.New("username already exists")
	}
	hashed, err := crypto.HashPasswordAsBcrypt(password)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		Username:            username,
		Password:            hashed,
		Role:                "reseller",
		UsageLimit:          usageLimit,
		AllowedInboundsMode: allowedInboundsMode,
		AllowedInboundIds:   allowedInboundIds,
		Multiplier:          multiplier,
	}
	if err := db.Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (s *ResellerService) UpdateReseller(id int, username, password string, usageLimit int64, allowedInboundsMode string, allowedInboundIds []int, multiplier float64) error {
	db := database.GetDB()
	user := &model.User{}
	if err := db.First(user, id).Error; err != nil {
		return err
	}
	if user.Role != "reseller" {
		return errors.New("user is not a reseller")
	}
	if multiplier < 1 {
		multiplier = 1
	}
	idsJSON, _ := json.Marshal(allowedInboundIds)
	updates := map[string]any{
		"username":              username,
		"usage_limit":           usageLimit,
		"allowed_inbounds_mode": allowedInboundsMode,
		"allowed_inbound_ids":   string(idsJSON),
		"multiplier":            multiplier,
	}
	if password != "" {
		hashed, err := crypto.HashPasswordAsBcrypt(password)
		if err != nil {
			return err
		}
		updates["password"] = hashed
	}
	if err := db.Model(user).Updates(updates).Error; err != nil {
		return err
	}
	if usageLimit == 0 || (user.UsageUp+user.UsageDown) < usageLimit {
		if rerr := s.ReEnableResellerClients(id); rerr != nil {
			logger.Warning("UpdateReseller: re-enable clients failed for reseller", id, ":", rerr)
		}
	}
	return nil
}

func (s *ResellerService) DeleteReseller(id int, deleteClients bool) error {
	db := database.GetDB()
	user := &model.User{}
	if err := db.First(user, id).Error; err != nil {
		return err
	}
	if user.Role != "reseller" {
		return errors.New("user is not a reseller")
	}
	if deleteClients {
		var emails []string
		db.Model(&model.ClientRecord{}).Where("owner_id = ?", id).Pluck("email", &emails)
		if len(emails) > 0 {
			db.Where("email IN ?", emails).Delete(&model.ClientRecord{})
			db.Table("client_traffics").Where("email IN ?", emails).Delete(nil)
			db.Where("client_id IN (SELECT id FROM clients WHERE owner_id = ?)", id).Delete(&model.ClientInbound{})
			db.Where("owner_id = ?", id).Delete(&model.Client{})
		}
	}
	return db.Delete(user).Error
}

func (s *ResellerService) ResetResellerUsage(id int) (int64, error) {
	db := database.GetDB()
	user := &model.User{}
	if err := db.First(user, id).Error; err != nil {
		return 0, err
	}
	if user.Role != "reseller" {
		return 0, errors.New("user is not a reseller")
	}

	if err := db.Model(&model.User{}).Where("id = ?", id).
		Updates(map[string]any{
			"usage_up":   0,
			"usage_down": 0,
		}).Error; err != nil {
		return 0, err
	}

	affected, _, err := s.reEnableLockedClients(id)
	return affected, err
}

// reEnableLockedClients restores connectivity for the clients this
// reseller's over-quota auto-disable locked, through the same
// ClientService.BulkSetEnable path every other enable/disable cascade in
// this file uses — a raw DB flip leaves the client removed from the live
// Xray config until an unrelated restart happens to pick it back up.
// Clients the reseller disabled manually (Locked=false) are left alone.
func (s *ResellerService) reEnableLockedClients(id int) (int64, bool, error) {
	db := database.GetDB()
	var emails []string
	db.Model(&model.ClientRecord{}).Where("owner_id = ? AND locked = ?", id, true).Pluck("email", &emails)
	if len(emails) == 0 {
		return 0, false, nil
	}
	if err := db.Model(&model.ClientRecord{}).Where("email IN ?", emails).Update("locked", false).Error; err != nil {
		return 0, false, err
	}
	_, needRestart, err := (&service.ClientService{}).BulkSetEnable(&service.InboundService{}, emails, true)
	if err != nil {
		return 0, false, err
	}
	return int64(len(emails)), needRestart, nil
}

func (s *ResellerService) ReEnableResellerClients(id int) error {
	_, _, err := s.reEnableLockedClients(id)
	return err
}

func (s *ResellerService) GetResellerById(id int) (*model.User, error) {
	db := database.GetDB()
	user := &model.User{}
	if err := db.First(user, id).Error; err != nil {
		return nil, err
	}
	return user, nil
}

// ToggleEnableReseller flips the reseller's own panel-login access AND
// cascades the same enable state to every client they've provisioned,
// through the same ClientService.BulkSetEnable path the normal per-client
// toggle uses — so a disabled reseller's clients actually stop working
// instead of just losing panel access while staying fully connectable.
//
// The cascade uses ClientRecord.Locked the same way the over-quota
// auto-disable job does: disabling marks only the clients it disabled as
// locked, and re-enabling only unlocks/re-enables those — a client the
// reseller had disabled manually beforehand (enable=false, locked=false)
// is left alone in both directions.
func (s *ResellerService) ToggleEnableReseller(id int) (bool, bool, error) {
	db := database.GetDB()
	user := &model.User{}
	if err := db.First(user, id).Error; err != nil {
		return false, false, err
	}
	if user.Role != "reseller" {
		return false, false, errors.New("user is not a reseller")
	}
	newEpoch := int64(0)
	if user.LoginEpoch >= 0 {
		newEpoch = -1
	}
	if err := db.Model(user).Update("login_epoch", newEpoch).Error; err != nil {
		return false, false, err
	}
	enabled := newEpoch >= 0

	if enabled {
		_, needRestart, err := s.reEnableLockedClients(id)
		if err != nil {
			logger.Warning("ToggleEnableReseller: re-enabling clients failed for reseller", id, ":", err)
			return enabled, false, nil
		}
		return enabled, needRestart, nil
	}

	var emails []string
	db.Model(&model.ClientRecord{}).Where("owner_id = ? AND enable = ?", id, true).Pluck("email", &emails)
	if len(emails) == 0 {
		return enabled, false, nil
	}
	_, needRestart, err := (&service.ClientService{}).BulkSetEnable(&service.InboundService{}, emails, false)
	if err != nil {
		logger.Warning("ToggleEnableReseller: cascading enable state to clients failed for reseller", id, ":", err)
		return enabled, false, nil
	}
	if err := db.Model(&model.ClientRecord{}).Where("email IN ?", emails).Update("locked", true).Error; err != nil {
		logger.Warning("ToggleEnableReseller: marking clients locked failed for reseller", id, ":", err)
	}
	return enabled, needRestart, nil
}

func (s *ResellerService) GetOverQuotaResellerIds(tx *gorm.DB) ([]int, error) {
	if tx == nil {
		db := database.GetDB()
		tx = db
	}
	var ids []int
	err := tx.Model(&model.User{}).
		Where("role = ? AND usage_limit > 0 AND (usage_up + usage_down >= usage_limit)", "reseller").
		Pluck("id", &ids).Error
	return ids, err
}
