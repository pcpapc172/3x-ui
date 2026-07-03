package panel

import (
	"errors"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"

	"gorm.io/gorm"
)

// ResellerInfo is the admin-facing projection of a reseller account.
type ResellerInfo struct {
	Id           int    `json:"id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	UsageLimit   int64  `json:"usageLimit"`
	UsageUp      int64  `json:"usageUp"`
	UsageDown    int64  `json:"usageDown"`
	ClientCount  int64  `json:"clientCount"`
	Enabled      bool   `json:"enabled"`
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
			Id:          u.Id,
			Username:    u.Username,
			Role:        u.Role,
			UsageLimit:  u.UsageLimit,
			UsageUp:     u.UsageUp,
			UsageDown:   u.UsageDown,
			ClientCount: count,
			Enabled:     u.LoginEpoch >= 0,
		})
	}
	return result, nil
}

func (s *ResellerService) CreateReseller(username, password string, usageLimit int64) (*model.User, error) {
	if username == "" {
		return nil, errors.New("username is required")
	}
	if password == "" {
		return nil, errors.New("password is required")
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
		Username:   username,
		Password:   hashed,
		Role:       "reseller",
		UsageLimit: usageLimit,
	}
	if err := db.Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (s *ResellerService) UpdateReseller(id int, username, password string, usageLimit int64) error {
	db := database.GetDB()
	user := &model.User{}
	if err := db.First(user, id).Error; err != nil {
		return err
	}
	if user.Role != "reseller" {
		return errors.New("user is not a reseller")
	}
	updates := map[string]any{
		"username":     username,
		"usage_limit":  usageLimit,
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
		s.ReEnableResellerClients(id)
	}
	return nil
}

func (s *ResellerService) DeleteReseller(id int) error {
	db := database.GetDB()
	user := &model.User{}
	if err := db.First(user, id).Error; err != nil {
		return err
	}
	if user.Role != "reseller" {
		return errors.New("user is not a reseller")
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

	result := db.Model(&model.User{}).Where("id = ?", id).
		Updates(map[string]any{
			"usage_up":   0,
			"usage_down": 0,
		})
	if result.Error != nil {
		return 0, result.Error
	}

	var emails []string
	db.Model(&model.ClientRecord{}).Where("owner_id = ?", id).Pluck("email", &emails)
	if len(emails) == 0 {
		return 0, nil
	}

	db.Model(&model.ClientRecord{}).Where("email IN ?", emails).Updates(map[string]any{"enable": true, "locked": false})
	db.Table("client_traffics").Where("email IN ?", emails).Update("enable", true)

	return int64(len(emails)), nil
}

func (s *ResellerService) ReEnableResellerClients(id int) error {
	db := database.GetDB()
	var emails []string
	db.Model(&model.ClientRecord{}).Where("owner_id = ? AND enable = ? AND locked = ?", id, false, false).Pluck("email", &emails)
	if len(emails) == 0 {
		return nil
	}
	db.Model(&model.ClientRecord{}).Where("email IN ?", emails).Update("enable", true)
	db.Table("client_traffics").Where("email IN ?", emails).Update("enable", true)

	return int64(len(emails)), nil
}

func (s *ResellerService) ReEnableResellerClients(id int) error {
	db := database.GetDB()
	var emails []string
	db.Model(&model.ClientRecord{}).Where("owner_id = ? AND enable = ? AND locked = ?", id, false, false).Pluck("email", &emails)
	if len(emails) == 0 {
		return nil
	}
	db.Model(&model.ClientRecord{}).Where("email IN ?", emails).Update("enable", true)
	db.Table("client_traffics").Where("email IN ?", emails).Update("enable", true)
	return nil
}

func (s *ResellerService) GetResellerById(id int) (*model.User, error) {
	db := database.GetDB()
	user := &model.User{}
	if err := db.First(user, id).Error; err != nil {
		return nil, err
	}
	return user, nil
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
