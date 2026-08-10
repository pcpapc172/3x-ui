package panel

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

// CreditWallet and DebitWallet are the only functions allowed to mutate
// BotCustomer.WalletBalanceToman; both run inside a locked transaction and
// always write a matching BotWalletTransaction ledger row alongside the
// balance change, so the balance is never mutated without an audit trail.

func (s *BotService) CreditWallet(ownerId, customerId int, kind string, amountToman int64, relatedOrderId int, note string) error {
	if amountToman <= 0 {
		return errors.New("credit amount must be positive")
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		var customer model.BotCustomer
		if err := database.LockForUpdate(tx).
			Where("id = ? AND owner_id = ?", customerId, ownerId).First(&customer).Error; err != nil {
			return err
		}
		newBalance := customer.WalletBalanceToman + amountToman
		if err := tx.Model(&customer).Update("wallet_balance_toman", newBalance).Error; err != nil {
			return err
		}
		return tx.Create(&model.BotWalletTransaction{
			OwnerId:           ownerId,
			CustomerId:        customerId,
			Kind:              kind,
			AmountToman:       amountToman,
			BalanceAfterToman: newBalance,
			RelatedOrderId:    relatedOrderId,
			Note:              note,
		}).Error
	})
}

func (s *BotService) DebitWallet(ownerId, customerId int, kind string, amountToman int64, relatedOrderId int, note string) error {
	if amountToman <= 0 {
		return errors.New("debit amount must be positive")
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		var customer model.BotCustomer
		if err := database.LockForUpdate(tx).
			Where("id = ? AND owner_id = ?", customerId, ownerId).First(&customer).Error; err != nil {
			return err
		}
		if customer.WalletBalanceToman < amountToman {
			return errors.New("insufficient wallet balance")
		}
		newBalance := customer.WalletBalanceToman - amountToman
		if err := tx.Model(&customer).Update("wallet_balance_toman", newBalance).Error; err != nil {
			return err
		}
		return tx.Create(&model.BotWalletTransaction{
			OwnerId:           ownerId,
			CustomerId:        customerId,
			Kind:              kind,
			AmountToman:       -amountToman,
			BalanceAfterToman: newBalance,
			RelatedOrderId:    relatedOrderId,
			Note:              note,
		}).Error
	})
}

// RedeemCode credits a customer's wallet from a gift code, consuming one use
// atomically (fails if the code was already exhausted by a concurrent redeem).
func (s *BotService) RedeemCode(ownerId, customerId int, code string) (int64, error) {
	var amount int64
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		var rc model.BotRedeemCode
		if err := database.LockForUpdate(tx).
			Where("owner_id = ? AND code = ?", ownerId, code).First(&rc).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("redeem code not found")
			}
			return err
		}
		if !rc.Enabled {
			return errors.New("redeem code is disabled")
		}
		if rc.ExpiresAt > 0 && rc.ExpiresAt < time.Now().UnixMilli() {
			return errors.New("redeem code has expired")
		}
		if rc.MaxUses > 0 && rc.UsedCount >= rc.MaxUses {
			return errors.New("redeem code has no uses left")
		}
		if err := tx.Model(&rc).Update("used_count", rc.UsedCount+1).Error; err != nil {
			return err
		}
		amount = rc.AmountToman
		return nil
	})
	if err != nil {
		return 0, err
	}
	if err := s.CreditWallet(ownerId, customerId, model.BotWalletTxRedeem, amount, 0, "redeem code "+code); err != nil {
		return 0, err
	}
	return amount, nil
}

func (s *BotService) ListDiscountCodes(ownerId int) ([]model.BotDiscountCode, error) {
	var codes []model.BotDiscountCode
	err := database.GetDB().Where("owner_id = ?", ownerId).Order("id DESC").Find(&codes).Error
	return codes, err
}

func (s *BotService) SaveDiscountCode(dc *model.BotDiscountCode) error {
	if dc.Code == "" {
		return errors.New("discount code is required")
	}
	if dc.Kind != "percent" && dc.Kind != "fixed" {
		return errors.New("discount kind must be percent or fixed")
	}
	if dc.Value <= 0 || (dc.Kind == "percent" && dc.Value > 100) {
		return errors.New("discount value out of range")
	}
	db := database.GetDB()
	if dc.Id == 0 {
		return db.Create(dc).Error
	}
	planIdsJSON, _ := json.Marshal(dc.PlanIds)
	return db.Model(&model.BotDiscountCode{}).Where("id = ? AND owner_id = ?", dc.Id, dc.OwnerId).Updates(map[string]any{
		"code":       dc.Code,
		"kind":       dc.Kind,
		"value":      dc.Value,
		"max_uses":   dc.MaxUses,
		"plan_ids":   string(planIdsJSON),
		"expires_at": dc.ExpiresAt,
		"enabled":    dc.Enabled,
	}).Error
}

func (s *BotService) DeleteDiscountCode(ownerId, id int) error {
	return database.GetDB().Where("id = ? AND owner_id = ?", id, ownerId).Delete(&model.BotDiscountCode{}).Error
}

// ValidateDiscountCode returns the Toman amount to subtract from priceToman
// for the given plan, or an error if the code doesn't apply.
func (s *BotService) ValidateDiscountCode(ownerId, planId int, code string, priceToman int64) (*model.BotDiscountCode, int64, error) {
	var dc model.BotDiscountCode
	err := database.GetDB().Where("owner_id = ? AND code = ?", ownerId, code).First(&dc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, errors.New("discount code not found")
		}
		return nil, 0, err
	}
	if !dc.Enabled {
		return nil, 0, errors.New("discount code is disabled")
	}
	if dc.ExpiresAt > 0 && dc.ExpiresAt < time.Now().UnixMilli() {
		return nil, 0, errors.New("discount code has expired")
	}
	if dc.MaxUses > 0 && dc.UsedCount >= dc.MaxUses {
		return nil, 0, errors.New("discount code has no uses left")
	}
	if len(dc.PlanIds) > 0 {
		allowed := false
		for _, id := range dc.PlanIds {
			if id == planId {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, 0, errors.New("discount code does not apply to this plan")
		}
	}
	var off int64
	if dc.Kind == "percent" {
		off = priceToman * dc.Value / 100
	} else {
		off = dc.Value
	}
	if off > priceToman {
		off = priceToman
	}
	return &dc, off, nil
}

// IncrementDiscountCodeUse is called once an order paid with this code is
// actually approved (not on mere application to an invoice, so an
// unpaid/canceled invoice doesn't burn a use).
func (s *BotService) IncrementDiscountCodeUse(id int) error {
	return database.GetDB().Model(&model.BotDiscountCode{}).
		Where("id = ?", id).
		UpdateColumn("used_count", gorm.Expr("used_count + 1")).Error
}

func (s *BotService) ListRedeemCodes(ownerId int) ([]model.BotRedeemCode, error) {
	var codes []model.BotRedeemCode
	err := database.GetDB().Where("owner_id = ?", ownerId).Order("id DESC").Find(&codes).Error
	return codes, err
}

func (s *BotService) SaveRedeemCode(rc *model.BotRedeemCode) error {
	if rc.Code == "" {
		return errors.New("redeem code is required")
	}
	if rc.AmountToman <= 0 {
		return errors.New("redeem amount must be positive")
	}
	db := database.GetDB()
	if rc.Id == 0 {
		return db.Create(rc).Error
	}
	return db.Model(&model.BotRedeemCode{}).Where("id = ? AND owner_id = ?", rc.Id, rc.OwnerId).Updates(map[string]any{
		"code":         rc.Code,
		"amount_toman": rc.AmountToman,
		"max_uses":     rc.MaxUses,
		"expires_at":   rc.ExpiresAt,
		"enabled":      rc.Enabled,
	}).Error
}

func (s *BotService) DeleteRedeemCode(ownerId, id int) error {
	return database.GetDB().Where("id = ? AND owner_id = ?", id, ownerId).Delete(&model.BotRedeemCode{}).Error
}
