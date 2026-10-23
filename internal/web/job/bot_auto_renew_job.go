package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/resellerbot"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
)

const (
	autoRenewWindowMs   = 24 * 60 * 60 * 1000 // renew once a client is within 24h of expiry
	autoRenewCooldownMs = 24 * 60 * 60 * 1000 // at most one attempt per subscription per day
)

// BotAutoRenewJob renews bot-sold subscriptions that have auto-renew
// enabled and are close to expiry, charging the buyer's wallet. Runs on a
// cadence via internal/web/web.go's startTask, alongside ResellerBotJob.
type BotAutoRenewJob struct {
	botService panel.BotService
}

func NewBotAutoRenewJob() *BotAutoRenewJob {
	return new(BotAutoRenewJob)
}

func (j *BotAutoRenewJob) Run() {
	enabled, err := (&service.SettingService{}).GetResellerBotEnabled()
	if err != nil || !enabled {
		return
	}

	subs, err := j.botService.DueForAutoRenew(autoRenewWindowMs, autoRenewCooldownMs)
	if err != nil {
		logger.Warning("bot auto-renew job: failed to list due subscriptions:", err)
		return
	}
	for _, sub := range subs {
		renewed, buyerTgId, err := j.botService.AttemptAutoRenew(sub)
		if err != nil {
			logger.Infof("bot auto-renew job: subscription %d not renewed: %v", sub.Id, err)
		}
		if buyerTgId != 0 {
			resellerbot.NotifyAutoRenewResult(sub.OwnerId, buyerTgId, renewed)
		}
	}
}
