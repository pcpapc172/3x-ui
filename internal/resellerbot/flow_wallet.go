package resellerbot

import (
	"strconv"
	"strings"

	"github.com/mymmrac/telego"
)

func (inst *botInstance) handleTopupPrompt(chatID int64, lang string) {
	inst.states.set(chatID, "awaiting_topup_amount")
	inst.send(chatID, t(lang, "walletTopupPrompt"))
}

func (inst *botInstance) handleTopupAmount(message telego.Message, lang string) {
	amount, err := strconv.ParseInt(strings.TrimSpace(message.Text), 10, 64)
	if err != nil || amount <= 0 {
		inst.send(message.Chat.ID, t(lang, "walletTopupInvalid"))
		return
	}
	inst.states.clear(message.Chat.ID)
	order, err := botSvc.CreateTopupOrder(inst.ownerId, message.From.ID, message.From.Username, amount, 0)
	if err != nil {
		inst.send(message.Chat.ID, t(lang, "error"))
		return
	}
	inst.sendInvoiceCard(message.Chat.ID, order.Id, lang)
}

func (inst *botInstance) handleRedeemPrompt(chatID int64, lang string) {
	inst.states.set(chatID, "awaiting_redeem_code")
	inst.send(chatID, t(lang, "redeemPrompt"))
}

func (inst *botInstance) handleRedeemCode(message telego.Message, lang string) {
	inst.states.clear(message.Chat.ID)
	customer, err := botSvc.GetOrCreateCustomer(inst.ownerId, message.From.ID, message.From.Username)
	if err != nil {
		inst.send(message.Chat.ID, t(lang, "error"))
		return
	}
	amount, err := botSvc.RedeemCode(inst.ownerId, customer.Id, strings.TrimSpace(message.Text))
	if err != nil {
		inst.send(message.Chat.ID, t(lang, "redeemFailed", err.Error()))
		return
	}
	inst.send(message.Chat.ID, t(lang, "redeemSuccess", amount))
}
