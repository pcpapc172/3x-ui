package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/resellerbot"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
)

// ResellerBotJob reconciles running per-owner Telegram sales-bot instances
// against enabled BotConfig rows (self-healing crashes and catching CRUD
// paths that missed an Ensure/Stop call) and expires stale pending orders.
type ResellerBotJob struct {
	botService panel.BotService
}

func NewResellerBotJob() *ResellerBotJob {
	return new(ResellerBotJob)
}

func (j *ResellerBotJob) Run() {
	resellerbot.GetManager().Reconcile()
	enabled, err := (&service.SettingService{}).GetResellerBotEnabled()
	if err != nil || !enabled {
		return
	}

	if n, err := j.botService.ExpireStalePendingOrders(); err != nil {
		logger.Warning("reseller bot job: expire stale orders failed:", err)
	} else if n > 0 {
		logger.Infof("reseller bot job: expired %d stale pending order(s)", n)
	}
}
