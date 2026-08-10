package panel

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// DueForAutoRenew returns subscriptions with auto-renew on whose client
// expires within withinMs and that haven't been attempted in the last
// cooldownMs — the cron job's candidate list.
func (s *BotService) DueForAutoRenew(withinMs, cooldownMs int64) ([]model.BotSubscription, error) {
	now := time.Now().UnixMilli()
	var subs []model.BotSubscription
	err := database.GetDB().
		Table("bot_subscriptions").
		Select("bot_subscriptions.*").
		Joins("JOIN client_traffics ON client_traffics.email = bot_subscriptions.client_email").
		Where("bot_subscriptions.auto_renew_enabled = ?", true).
		Where("client_traffics.expiry_time > 0 AND client_traffics.expiry_time <= ?", now+withinMs).
		Where("bot_subscriptions.last_auto_renew_attempt < ?", now-cooldownMs).
		Find(&subs).Error
	return subs, err
}

// AttemptAutoRenew creates a renewal invoice for the subscription's own plan
// and pays it from the buyer's wallet immediately (auto-renew has no owner-
// review step by design). Always stamps LastAutoRenewAttempt, success or
// failure, so DueForAutoRenew won't retry the same subscription within
// cooldownMs regardless of outcome. Returns the buyer's chat id so the
// caller can notify them even on failure.
func (s *BotService) AttemptAutoRenew(sub model.BotSubscription) (renewed bool, buyerTgId int64, err error) {
	db := database.GetDB()
	defer func() {
		db.Model(&model.BotSubscription{}).Where("id = ?", sub.Id).
			Update("last_auto_renew_attempt", time.Now().UnixMilli())
	}()

	customer := &model.BotCustomer{}
	if cErr := db.First(customer, sub.CustomerId).Error; cErr != nil {
		return false, 0, cErr
	}
	order, cErr := s.CreateRenewalOrder(sub.OwnerId, sub.Id, customer.BuyerTgId, customer.BuyerUsername, 0, "")
	if cErr != nil {
		return false, customer.BuyerTgId, cErr
	}
	if _, _, pErr := s.PayOrderFromWallet(order.Id); pErr != nil {
		_ = s.CancelOrder(sub.OwnerId, order.Id)
		return false, customer.BuyerTgId, pErr
	}
	return true, customer.BuyerTgId, nil
}
