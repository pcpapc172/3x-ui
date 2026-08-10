package resellerbot

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
)

func (inst *botInstance) notifyBuyerApproved(order *model.BotOrder, email string) {
	inst.send(order.BuyerTgId, fmt.Sprintf("%s\n\n%s: %s", t("botshop.orderApproved"), t("botshop.yourClient"), email))
}

func (inst *botInstance) notifyBuyerRejected(order *model.BotOrder) {
	inst.send(order.BuyerTgId, t("botshop.orderRejected"))
}

func (inst *botInstance) notifyOwnerPendingReview(orderID int) {
	cfg, err := configFor(inst.ownerId)
	if err != nil || cfg.OwnerChatId == 0 {
		return
	}
	inst.send(cfg.OwnerChatId, fmt.Sprintf("%s #%d", t("botshop.newOrderPendingReview"), orderID))
}

// NotifyOrderApproved messages the buyer through the owner's bot after an
// order is approved from the web panel. A no-op if that bot isn't running
// (e.g. disabled) — the buyer just won't get a Telegram ping.
func NotifyOrderApproved(order *model.BotOrder, email string) {
	if inst, ok := GetManager().instanceFor(order.OwnerId); ok {
		inst.notifyBuyerApproved(order, email)
	}
}

// NotifyOrderRejected messages the buyer through the owner's bot after an
// order is rejected from the web panel.
func NotifyOrderRejected(order *model.BotOrder) {
	if inst, ok := GetManager().instanceFor(order.OwnerId); ok {
		inst.notifyBuyerRejected(order)
	}
}

// FetchProofPhoto streams a payment-proof photo live from Telegram by
// file_id — nothing is ever stored locally. Uses the owner's running bot
// instance if there is one, otherwise spins up a throwaway client from the
// stored token (the bot may be temporarily stopped while still configured).
func FetchProofPhoto(ownerId int, fileId string) (io.ReadCloser, string, error) {
	bot, err := botClientFor(ownerId)
	if err != nil {
		return nil, "", err
	}
	file, err := bot.GetFile(context.Background(), &telego.GetFileParams{FileID: fileId})
	if err != nil {
		return nil, "", err
	}
	url := bot.FileDownloadURL(file.FilePath)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, "", fmt.Errorf("telegram file download returned %s", resp.Status)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return resp.Body, contentType, nil
}

func botClientFor(ownerId int) (*telego.Bot, error) {
	if inst, ok := GetManager().instanceFor(ownerId); ok {
		return inst.bot, nil
	}
	cfg, err := configFor(ownerId)
	if err != nil || cfg.Token == "" {
		return nil, fmt.Errorf("no bot token configured for owner %d", ownerId)
	}
	bot, err := telego.NewBot(cfg.Token)
	if err != nil {
		logger.Warningf("resellerbot: throwaway bot client failed for owner %d: %v", ownerId, err)
	}
	return bot, err
}
