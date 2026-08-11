package panel

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

// SubscriptionStats is the live traffic/expiry snapshot for a subscription's
// underlying client, read from xray.ClientTraffic (not the Bot* tables).
type SubscriptionStats struct {
	Up, Down, Total, ExpiryTime int64
	Enable                      bool
}

func (s *BotService) GetSubscriptionStats(clientEmail string) (*SubscriptionStats, error) {
	var tr xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", clientEmail).First(&tr).Error; err != nil {
		return nil, err
	}
	return &SubscriptionStats{Up: tr.Up, Down: tr.Down, Total: tr.Total, ExpiryTime: tr.ExpiryTime, Enable: tr.Enable}, nil
}

func (s *BotService) ListSubscriptions(ownerId, customerId int) ([]model.BotSubscription, error) {
	var subs []model.BotSubscription
	err := database.GetDB().Where("owner_id = ? AND customer_id = ?", ownerId, customerId).
		Order("id DESC").Find(&subs).Error
	return subs, err
}

func (s *BotService) GetSubscription(ownerId, id int) (*model.BotSubscription, error) {
	sub := &model.BotSubscription{}
	err := database.GetDB().Where("id = ? AND owner_id = ?", id, ownerId).First(sub).Error
	return sub, err
}

// GetSubscriptionByEmail looks up a subscription by its client's email —
// used right after a purchase/renewal settles, when the caller only has the
// provisioned/renewed client's email, not the BotSubscription id.
func (s *BotService) GetSubscriptionByEmail(ownerId int, email string) (*model.BotSubscription, error) {
	sub := &model.BotSubscription{}
	err := database.GetDB().Where("owner_id = ? AND client_email = ?", ownerId, email).First(sub).Error
	return sub, err
}

// GetSubscriptionConfigs builds the subscription URL for a client, reusing
// the same host/path/TLS resolution the admin Telegram bot and web panel
// already use (service.BuildSubscriptionURLs) rather than re-deriving it.
func (s *BotService) GetSubscriptionConfigs(clientEmail string) (subURL string, err error) {
	inboundSvc := &service.InboundService{}
	settingSvc := &service.SettingService{}
	subURL, _, err = service.BuildSubscriptionURLs(inboundSvc, settingSvc, clientEmail, "")
	return subURL, err
}

// GetClientConfigLinks returns the client's individual per-protocol share
// links (vless://, vmess://, trojan://, ...), one per attached inbound —
// what "Get configs" actually hands the buyer, as opposed to the aggregate
// /sub/<subId> URL from GetSubscriptionConfigs.
func (s *BotService) GetClientConfigLinks(clientEmail string) ([]string, error) {
	inboundSvc := &service.InboundService{}
	settingSvc := &service.SettingService{}
	host, _ := service.ResolveFallbackHost(settingSvc, "")
	return inboundSvc.GetAllClientLinks(host, clientEmail)
}

// RevokeLink issues fresh credentials for the subscription's client —
// invalidating every previously shared link/config — without touching
// traffic, quota, or expiry. See ClientService.RegenerateCredentialsByEmail.
func (s *BotService) RevokeLink(ownerId, subscriptionId int) (newSubID string, err error) {
	sub, err := s.GetSubscription(ownerId, subscriptionId)
	if err != nil {
		return "", err
	}
	inboundSvc := &service.InboundService{}
	clientSvc := &service.ClientService{}
	_, newSubID, err = clientSvc.RegenerateCredentialsByEmail(inboundSvc, sub.ClientEmail)
	return newSubID, err
}

func (s *BotService) SetAutoRenew(ownerId, subscriptionId int, enabled bool) error {
	result := database.GetDB().Model(&model.BotSubscription{}).
		Where("id = ? AND owner_id = ?", subscriptionId, ownerId).
		Update("auto_renew_enabled", enabled)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
