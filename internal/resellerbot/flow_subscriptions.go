package resellerbot

import (
	"fmt"
	"html"
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// timeLeftLabel renders remaining time as "Xd Xh" ("∞" unlimited, "0" past
// expiry) — a short local variant of internal/sub's richer formatter, not
// worth sharing across packages for this much logic.
func timeLeftLabel(expiryMs int64) string {
	if expiryMs == 0 {
		return "∞"
	}
	exp := expiryMs / 1000
	var secs int64
	if exp > 0 {
		secs = exp - time.Now().Unix()
	} else {
		secs = -exp
	}
	if secs <= 0 {
		return "0"
	}
	days := secs / 86400
	hours := (secs % 86400) / 3600
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	return fmt.Sprintf("%dh", hours)
}

func (inst *botInstance) sendSubscriptionList(chatID, buyerTgID int64, buyerUsername, lang string) {
	customer, err := botSvc.GetOrCreateCustomer(inst.ownerId, buyerTgID, buyerUsername)
	if err != nil {
		inst.send(chatID, t(lang, "error"))
		return
	}
	subs, err := botSvc.ListSubscriptions(inst.ownerId, customer.Id)
	if err != nil || len(subs) == 0 {
		inst.send(chatID, t(lang, "subListEmpty"))
		return
	}
	var rows [][]telego.InlineKeyboardButton
	for _, sub := range subs {
		label := sub.ClientEmail
		if plan, pErr := botSvc.GetPlan(inst.ownerId, sub.PlanId); pErr == nil {
			label = plan.Name
		}
		rows = append(rows, tu.InlineKeyboardRow(tu.InlineKeyboardButton(label).WithCallbackData("sub:view:"+strconv.Itoa(sub.Id))))
	}
	inst.send(chatID, t(lang, "menuSubs"), tu.InlineKeyboard(rows...))
}

// clientStatusLabel collapses enable/expiry into the same three states the
// subscription-page/subscription bot card shows.
func clientStatusLabel(lang string, stats *panel.SubscriptionStats) string {
	switch {
	case stats == nil || !stats.Enable:
		return t(lang, "clientStatusDisabled")
	case stats.ExpiryTime > 0 && stats.ExpiryTime/1000 < time.Now().Unix():
		return t(lang, "clientStatusExpired")
	default:
		return t(lang, "clientStatusActive")
	}
}

// subscriptionCard renders the full subscription-detail message (status,
// username, usage, remaining time, sub link) and its action buttons —
// shared by "my subscriptions" and the post-purchase/renewal delivery card.
func subscriptionCard(sub *model.BotSubscription, lang string) (string, *telego.InlineKeyboardMarkup) {
	stats, _ := botSvc.GetSubscriptionStats(sub.ClientEmail)
	subURL, _ := botSvc.GetSubscriptionConfigs(sub.ClientEmail)

	text := t(lang, "subStatus", clientStatusLabel(lang, stats)) + "\n" + t(lang, "subUsername", sub.ClientEmail)
	if stats != nil {
		used := common.FormatTraffic(stats.Up + stats.Down)
		remaining := t(lang, "unlimited")
		if stats.Total > 0 {
			remaining = common.FormatTraffic(stats.Total - stats.Up - stats.Down)
		}
		text += "\n" + t(lang, "subUsed", used) + "\n" + t(lang, "subRemainingQuota", remaining) +
			"\n" + t(lang, "subRemaining", timeLeftLabel(stats.ExpiryTime))
	}
	if subURL != "" {
		text += "\n\n" + t(lang, "subLink", subURL)
	}

	autoLabel := "autoRenewOff"
	if sub.AutoRenewEnabled {
		autoLabel = "autoRenewOn"
	}
	idStr := strconv.Itoa(sub.Id)
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(tu.InlineKeyboardButton(t(lang, "btnGetConfigs")).WithCallbackData("sub:configs:" + idStr)),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton(t(lang, "btnRevoke")).WithCallbackData("sub:revoke:" + idStr)),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton(t(lang, "btnRenew")).WithCallbackData("sub:renew:" + idStr)),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton(t(lang, autoLabel)).WithCallbackData("sub:autorenew:" + idStr)),
	}
	return text, tu.InlineKeyboard(rows...)
}

func (inst *botInstance) sendSubscriptionDetail(chatID int64, subID int, lang string) {
	sub, err := botSvc.GetSubscription(inst.ownerId, subID)
	if err != nil {
		inst.send(chatID, t(lang, "error"))
		return
	}
	text, kb := subscriptionCard(sub, lang)
	inst.send(chatID, text, kb)
}

func (inst *botInstance) handleGetConfigs(chatID int64, subIDStr, lang string) {
	subID, err := strconv.Atoi(subIDStr)
	if err != nil {
		return
	}
	sub, err := botSvc.GetSubscription(inst.ownerId, subID)
	if err != nil {
		inst.send(chatID, t(lang, "error"))
		return
	}
	links, err := botSvc.GetClientConfigLinks(sub.ClientEmail)
	if err != nil || len(links) == 0 {
		inst.send(chatID, t(lang, "error"))
		return
	}
	for _, link := range links {
		inst.send(chatID, "<code>"+html.EscapeString(link)+"</code>")
	}
}

func (inst *botInstance) handleRevokePrompt(chatID int64, subIDStr, lang string) {
	inst.send(chatID, t(lang, "revokeConfirm"), tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t(lang, "btnRevoke")).WithCallbackData("sub:revokeconfirm:"+subIDStr),
	)))
}

func (inst *botInstance) handleRevokeConfirm(chatID int64, subIDStr, lang string) {
	subID, err := strconv.Atoi(subIDStr)
	if err != nil {
		return
	}
	if _, err := botSvc.RevokeLink(inst.ownerId, subID); err != nil {
		inst.send(chatID, t(lang, "error"))
		return
	}
	inst.send(chatID, t(lang, "revokeDone"))
}

func (inst *botInstance) handleToggleAutoRenew(chatID int64, subIDStr, lang string) {
	subID, err := strconv.Atoi(subIDStr)
	if err != nil {
		return
	}
	sub, err := botSvc.GetSubscription(inst.ownerId, subID)
	if err != nil {
		inst.send(chatID, t(lang, "error"))
		return
	}
	if err := botSvc.SetAutoRenew(inst.ownerId, subID, !sub.AutoRenewEnabled); err != nil {
		inst.send(chatID, t(lang, "error"))
		return
	}
	inst.send(chatID, t(lang, "autoRenewToggled"))
	inst.sendSubscriptionDetail(chatID, subID, lang)
}
