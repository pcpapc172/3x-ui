package panel

import (
	"errors"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"gorm.io/gorm"
)

// IsClientEmailTaken checks whether a buyer-chosen username is already used
// by another client, so the bot can reject it before an order is even
// created (ClientService.Create enforces this too, at settlement time —
// this is just the earlier, friendlier check).
func (s *BotService) IsClientEmailTaken(email string) (bool, error) {
	clientSvc := &service.ClientService{}
	_, err := clientSvc.GetRecordByEmail(nil, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ListCustomers is the admin-facing "Customers" tab — every buyer who has
// ever contacted this owner's bot.
func (s *BotService) ListCustomers(ownerId int) ([]model.BotCustomer, error) {
	var customers []model.BotCustomer
	err := database.GetDB().Where("owner_id = ?", ownerId).Order("id DESC").Find(&customers).Error
	return customers, err
}

// AdjustWallet is the manual support-side wallet correction the Customers
// tab exposes; deltaToman may be negative. Routed through CreditWallet/
// DebitWallet so it's ledgered the same as every other balance change.
func (s *BotService) AdjustWallet(ownerId, customerId int, deltaToman int64, note string) error {
	switch {
	case deltaToman > 0:
		return s.CreditWallet(ownerId, customerId, model.BotWalletTxAdminAdjust, deltaToman, 0, note)
	case deltaToman < 0:
		return s.DebitWallet(ownerId, customerId, model.BotWalletTxAdminAdjust, -deltaToman, 0, note)
	default:
		return errors.New("amount must be non-zero")
	}
}

// GetOrCreateCustomer returns the buyer's persistent profile for this
// owner's bot, creating one (with a fresh referral code) on first contact.
func (s *BotService) GetOrCreateCustomer(ownerId int, buyerTgId int64, buyerUsername string) (*model.BotCustomer, error) {
	db := database.GetDB()
	c := &model.BotCustomer{}
	err := db.Where("owner_id = ? AND buyer_tg_id = ?", ownerId, buyerTgId).First(c).Error
	if err == nil {
		if buyerUsername != "" && buyerUsername != c.BuyerUsername {
			if uErr := db.Model(c).Update("buyer_username", buyerUsername).Error; uErr != nil {
				return nil, uErr
			}
			c.BuyerUsername = buyerUsername
		}
		return c, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	code, err := s.uniqueReferralCode(ownerId)
	if err != nil {
		return nil, err
	}
	c = &model.BotCustomer{
		OwnerId:              ownerId,
		BuyerTgId:            buyerTgId,
		BuyerUsername:        buyerUsername,
		ReferralCode:         code,
		NotificationsEnabled: true,
	}
	if err := db.Create(c).Error; err != nil {
		return nil, err
	}
	return c, nil
}

func (s *BotService) uniqueReferralCode(ownerId int) (string, error) {
	db := database.GetDB()
	for i := 0; i < 10; i++ {
		code := random.Seq(8)
		var count int64
		if err := db.Model(&model.BotCustomer{}).
			Where("owner_id = ? AND referral_code = ?", ownerId, code).
			Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", errors.New("failed to generate a unique referral code")
}

// GetCustomerByReferralCode looks up a referrer by their code, scoped to one
// owner. Returns (nil, nil) when no match exists.
func (s *BotService) GetCustomerByReferralCode(ownerId int, code string) (*model.BotCustomer, error) {
	c := &model.BotCustomer{}
	err := database.GetDB().Where("owner_id = ? AND referral_code = ?", ownerId, code).First(c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return c, err
}

// SetReferrer links a brand-new customer to whoever referred them. A no-op
// once already set, or if a customer tries to refer themselves.
func (s *BotService) SetReferrer(customerId, referrerId int) error {
	if customerId == referrerId || referrerId == 0 {
		return nil
	}
	return database.GetDB().Model(&model.BotCustomer{}).
		Where("id = ? AND referred_by_customer_id = 0", customerId).
		Update("referred_by_customer_id", referrerId).Error
}

func (s *BotService) SetLanguage(customerId int, lang string) error {
	if lang != "fa" && lang != "en" {
		return errors.New("unsupported language")
	}
	return database.GetDB().Model(&model.BotCustomer{}).Where("id = ?", customerId).Update("language", lang).Error
}

func (s *BotService) SetNotificationsEnabled(customerId int, enabled bool) error {
	return database.GetDB().Model(&model.BotCustomer{}).Where("id = ?", customerId).
		Update("notifications_enabled", enabled).Error
}

// ReferralStats reports how many customers this one referred and how much
// bonus they've earned so far.
type ReferralStats struct {
	ReferralCode    string
	ReferredCount   int64
	TotalBonusToman int64
}

func (s *BotService) ReferralStats(customerId int) (*ReferralStats, error) {
	db := database.GetDB()
	c := &model.BotCustomer{}
	if err := db.First(c, customerId).Error; err != nil {
		return nil, err
	}
	var count int64
	if err := db.Model(&model.BotCustomer{}).
		Where("referred_by_customer_id = ?", customerId).Count(&count).Error; err != nil {
		return nil, err
	}
	var totalBonus int64
	if err := db.Model(&model.BotWalletTransaction{}).
		Where("customer_id = ? AND kind = ?", customerId, model.BotWalletTxReferralBonus).
		Select("COALESCE(SUM(amount_toman), 0)").Scan(&totalBonus).Error; err != nil {
		return nil, err
	}
	return &ReferralStats{ReferralCode: c.ReferralCode, ReferredCount: count, TotalBonusToman: totalBonus}, nil
}

// ApplyReferralBonus credits the referrer's wallet per the owner's configured
// ReferralKind/ReferralValue/ReferralTrigger. Called only for Kind=="purchase"
// orders once approved — renewals and wallet top-ups never earn a referral.
func (s *BotService) ApplyReferralBonus(order *model.BotOrder) error {
	cfg, err := s.GetConfig(order.OwnerId)
	if err != nil {
		return err
	}
	if !cfg.ReferralEnabled {
		return nil
	}
	buyer, err := s.GetOrCreateCustomer(order.OwnerId, order.BuyerTgId, order.BuyerUsername)
	if err != nil {
		return err
	}
	if buyer.ReferredByCustomerId == 0 {
		return nil
	}
	if cfg.ReferralTrigger == "first_purchase" {
		var priorApproved int64
		if err := database.GetDB().Model(&model.BotOrder{}).
			Where("owner_id = ? AND buyer_tg_id = ? AND status = ? AND kind = ? AND id <> ?",
				order.OwnerId, order.BuyerTgId, model.BotOrderApproved, model.BotOrderKindPurchase, order.Id).
			Count(&priorApproved).Error; err != nil {
			return err
		}
		if priorApproved > 0 {
			return nil
		}
	}
	var bonus int64
	if cfg.ReferralKind == "percent" {
		bonus = order.FinalAmountToman * cfg.ReferralValue / 100
	} else {
		bonus = cfg.ReferralValue
	}
	if bonus <= 0 {
		return nil
	}
	return s.CreditWallet(order.OwnerId, buyer.ReferredByCustomerId, model.BotWalletTxReferralBonus,
		bonus, order.Id, "referral bonus")
}
