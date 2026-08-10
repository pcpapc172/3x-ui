package panel

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"

	"gorm.io/gorm"
)

const (
	bytesPerGB    = 1024 * 1024 * 1024
	msPerDay      = 24 * 60 * 60 * 1000
	staleOrderTTL = 30 * time.Minute
)

// BotService owns the DB-backed configuration, plans, payment methods and
// order review for one owner's Telegram sales bot. Order creation happens
// here too (called from internal/resellerbot's router) so every DB write for
// this feature lives in one place; the bot process lifecycle itself is
// owned by internal/resellerbot.Manager, not this package.
type BotService struct{}

func (s *BotService) GetConfig(ownerId int) (*model.BotConfig, error) {
	db := database.GetDB()
	cfg := &model.BotConfig{}
	err := db.Where("owner_id = ?", ownerId).First(cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &model.BotConfig{OwnerId: ownerId}, nil
	}
	return cfg, err
}

// BotConfigInput carries the owner-editable Setup-tab fields, including the
// wallet/referral knobs that make the referral system "fully configurable".
type BotConfigInput struct {
	Token           string
	Enabled         bool
	WelcomeMessage  string
	WalletEnabled   bool
	ReferralEnabled bool
	ReferralKind    string
	ReferralValue   int64
	ReferralTrigger string
	SupportText     string
	TutorialText    string
}

func (s *BotService) SaveConfig(ownerId int, in BotConfigInput) (*model.BotConfig, error) {
	db := database.GetDB()
	cfg := &model.BotConfig{}
	err := db.Where("owner_id = ?", ownerId).First(cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		cfg = &model.BotConfig{OwnerId: ownerId}
		if err := db.Create(cfg).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if in.ReferralKind == "" {
		in.ReferralKind = "flat"
	}
	if in.ReferralTrigger == "" {
		in.ReferralTrigger = "first_purchase"
	}
	if in.ReferralKind != "flat" && in.ReferralKind != "percent" {
		return nil, errors.New("referral kind must be flat or percent")
	}
	if in.ReferralTrigger != "first_purchase" && in.ReferralTrigger != "every_purchase" {
		return nil, errors.New("referral trigger must be first_purchase or every_purchase")
	}
	updates := map[string]any{
		"enabled":          in.Enabled,
		"welcome_message":  in.WelcomeMessage,
		"wallet_enabled":   in.WalletEnabled,
		"referral_enabled": in.ReferralEnabled,
		"referral_kind":    in.ReferralKind,
		"referral_value":   in.ReferralValue,
		"referral_trigger": in.ReferralTrigger,
		"support_text":     in.SupportText,
		"tutorial_text":    in.TutorialText,
	}
	if in.Token != "" {
		updates["token"] = in.Token
	}
	if err := db.Model(cfg).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetConfig(ownerId)
}

// GenerateLinkCode mints a fresh one-time code the owner sends to their own
// bot as "/link <code>" to register their chat as the order-review target.
func (s *BotService) GenerateLinkCode(ownerId int) (string, error) {
	code := random.Seq(8)
	db := database.GetDB()
	cfg, err := s.GetConfig(ownerId)
	if err != nil {
		return "", err
	}
	if cfg.Id == 0 {
		cfg = &model.BotConfig{OwnerId: ownerId, LinkCode: code}
		if err := db.Create(cfg).Error; err != nil {
			return "", err
		}
		return code, nil
	}
	if err := db.Model(&model.BotConfig{}).Where("owner_id = ?", ownerId).Update("link_code", code).Error; err != nil {
		return "", err
	}
	return code, nil
}

// VerifyLinkCode is called from the owner's own running bot instance when it
// receives "/link <code>". A match registers chatId as the review target and
// consumes the code.
func (s *BotService) VerifyLinkCode(ownerId int, code string, chatId int64) (bool, error) {
	if code == "" {
		return false, nil
	}
	db := database.GetDB()
	result := db.Model(&model.BotConfig{}).
		Where("owner_id = ? AND link_code = ?", ownerId, code).
		Updates(map[string]any{"owner_chat_id": chatId, "link_code": ""})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (s *BotService) ListPlans(ownerId int) ([]model.BotPlan, error) {
	var plans []model.BotPlan
	err := database.GetDB().Where("owner_id = ?", ownerId).Order("sort_order ASC, id ASC").Find(&plans).Error
	return plans, err
}

func (s *BotService) GetPlan(ownerId, id int) (*model.BotPlan, error) {
	plan := &model.BotPlan{}
	err := database.GetDB().Where("id = ? AND owner_id = ?", id, ownerId).First(plan).Error
	return plan, err
}

func (s *BotService) SavePlan(plan *model.BotPlan) error {
	if plan.Name == "" {
		return errors.New("plan name is required")
	}
	if plan.TrafficGB < 0 || plan.DurationDays <= 0 || plan.PriceToman < 0 {
		return errors.New("plan traffic/duration/price out of range")
	}
	if len(plan.InboundIds) == 0 {
		return errors.New("at least one inbound is required")
	}
	db := database.GetDB()
	if plan.IsTrial {
		// At most one trial plan per owner — demote any other.
		if err := db.Model(&model.BotPlan{}).
			Where("owner_id = ? AND id <> ? AND is_trial = ?", plan.OwnerId, plan.Id, true).
			Update("is_trial", false).Error; err != nil {
			return err
		}
	}
	if plan.Id == 0 {
		return db.Create(plan).Error
	}
	idsJSON, _ := json.Marshal(plan.InboundIds)
	return db.Model(&model.BotPlan{}).Where("id = ? AND owner_id = ?", plan.Id, plan.OwnerId).Updates(map[string]any{
		"name":          plan.Name,
		"traffic_gb":    plan.TrafficGB,
		"duration_days": plan.DurationDays,
		"price_toman":   plan.PriceToman,
		"inbound_ids":   string(idsJSON),
		"enabled":       plan.Enabled,
		"sort_order":    plan.SortOrder,
		"is_trial":      plan.IsTrial,
	}).Error
}

func (s *BotService) DeletePlan(ownerId, id int) error {
	return database.GetDB().Where("id = ? AND owner_id = ?", id, ownerId).Delete(&model.BotPlan{}).Error
}

func (s *BotService) ListPaymentMethods(ownerId int) ([]model.BotPaymentMethod, error) {
	var methods []model.BotPaymentMethod
	err := database.GetDB().Where("owner_id = ?", ownerId).Order("sort_order ASC, id ASC").Find(&methods).Error
	return methods, err
}

func (s *BotService) GetPaymentMethod(ownerId, id int) (*model.BotPaymentMethod, error) {
	method := &model.BotPaymentMethod{}
	err := database.GetDB().Where("id = ? AND owner_id = ?", id, ownerId).First(method).Error
	return method, err
}

func (s *BotService) SavePaymentMethod(method *model.BotPaymentMethod) error {
	if method.Type != "card" && method.Type != "crypto" {
		return errors.New("payment method type must be card or crypto")
	}
	if method.Label == "" || method.Value == "" {
		return errors.New("payment method label and value are required")
	}
	db := database.GetDB()
	if method.Id == 0 {
		return db.Create(method).Error
	}
	return db.Model(&model.BotPaymentMethod{}).Where("id = ? AND owner_id = ?", method.Id, method.OwnerId).Updates(map[string]any{
		"type":        method.Type,
		"label":       method.Label,
		"value":       method.Value,
		"holder_name": method.HolderName,
		"enabled":     method.Enabled,
		"sort_order":  method.SortOrder,
	}).Error
}

func (s *BotService) DeletePaymentMethod(ownerId, id int) error {
	return database.GetDB().Where("id = ? AND owner_id = ?", id, ownerId).Delete(&model.BotPaymentMethod{}).Error
}

// Order lifecycle (ListOrders/GetOrder/CreateOrder variants/SubmitProof/
// CancelOrder/ApproveOrder/RejectOrder/ExpireStalePendingOrders) lives in
// reseller_bot_orders.go.
