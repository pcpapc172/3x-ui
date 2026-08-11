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

// notifyBuyerApproved sends the full subscription-detail card (not just a
// bare "approved" line) — the buyer's actual delivery message for a fresh
// purchase or a renewal.
func (inst *botInstance) notifyBuyerApproved(order *model.BotOrder, email string) {
	lang := inst.customerLang(order.BuyerTgId, order.BuyerUsername)
	if email == "" {
		return // topup: settleOrder already messaged the buyer via redeemSuccess-style text
	}
	sub, err := botSvc.GetSubscriptionByEmail(order.OwnerId, email)
	if err != nil {
		inst.send(order.BuyerTgId, t(lang, "orderApproved", email))
		return
	}
	text, kb := subscriptionCard(sub, lang)
	inst.send(order.BuyerTgId, t(lang, "subscriptionActivated")+"\n\n"+text, kb)
}

func (inst *botInstance) notifyBuyerRejected(order *model.BotOrder) {
	lang := inst.customerLang(order.BuyerTgId, order.BuyerUsername)
	inst.send(order.BuyerTgId, t(lang, "orderRejected"))
}

// notifyOwnerPendingReview pushes the same proof+Approve/Reject card /orders
// shows, directly to the owner's linked chat, so a new order doesn't require
// the owner to go run /orders manually to see and act on it.
func (inst *botInstance) notifyOwnerPendingReview(orderID int) {
	cfg, err := configFor(inst.ownerId)
	if err != nil || cfg.OwnerChatId == 0 {
		return
	}
	order, err := botSvc.GetOrder(inst.ownerId, orderID)
	if err != nil {
		return
	}
	inst.send(cfg.OwnerChatId, t(langFA, "newOrderPendingReview"))
	inst.sendOrderReviewCard(cfg.OwnerChatId, *order)
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

// NotifyAutoRenewResult messages the buyer after the auto-renew cron job
// attempts their subscription, success or failure (insufficient balance).
func NotifyAutoRenewResult(ownerId int, buyerTgId int64, renewed bool) {
	inst, ok := GetManager().instanceFor(ownerId)
	if !ok {
		return
	}
	lang := inst.customerLang(buyerTgId, "")
	key := "autoRenewFailedBalance"
	if renewed {
		key = "autoRenewSucceeded"
	}
	inst.send(buyerTgId, t(lang, key))
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
