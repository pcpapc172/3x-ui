package resellerbot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

var botSvc = &panel.BotService{}

func registerHandlers(h *th.BotHandler, inst *botInstance) {
	h.HandleMessage(inst.handleStart, th.CommandEqual("start"))
	h.HandleMessage(inst.handleOrdersCommand, th.CommandEqual("orders"))
	h.HandleMessage(inst.handleLinkCommand, th.CommandPrefix("link"))
	h.HandleCallbackQuery(inst.handleCallback, th.AnyCallbackQueryWithMessage())
	h.HandleMessage(inst.handleMessage, th.AnyMessage())
}

// customerLang resolves the buyer's chosen language, defaulting to Persian
// for a brand-new contact (matches BotCustomer.Language's "" == unset
// convention) — used by every buyer-facing callback/message handler.
func (inst *botInstance) customerLang(buyerTgID int64, buyerUsername string) string {
	customer, err := botSvc.GetOrCreateCustomer(inst.ownerId, buyerTgID, buyerUsername)
	if err != nil || customer.Language == "" {
		return langFA
	}
	return customer.Language
}

func (inst *botInstance) handleLinkCommand(_ *th.Context, message telego.Message) error {
	fields := strings.Fields(message.Text)
	if len(fields) < 2 {
		inst.send(message.Chat.ID, t(langFA, "linkUsage"))
		return nil
	}
	ok, err := botSvc.VerifyLinkCode(inst.ownerId, fields[1], message.Chat.ID)
	if err != nil || !ok {
		inst.send(message.Chat.ID, t(langFA, "linkFailed"))
		return nil
	}
	inst.send(message.Chat.ID, t(langFA, "linkSuccess"))
	return nil
}

func (inst *botInstance) handleOrdersCommand(_ *th.Context, message telego.Message) error {
	if !isOwnerChat(inst.ownerId, message.Chat.ID) {
		return nil
	}
	orders, err := botSvc.ListOrders(inst.ownerId, model.BotOrderPendingReview)
	if err != nil || len(orders) == 0 {
		inst.send(message.Chat.ID, t(langFA, "noPendingOrders"))
		return nil
	}
	for _, o := range orders {
		inst.sendOrderReviewCard(message.Chat.ID, o)
	}
	return nil
}

// sendOrderReviewCard renders one pending_review order for the owner: plan,
// buyer, proof (photo or text), and inline Approve/Reject buttons — used by
// both /orders and the pending-review push notification, so an owner never
// has to run /orders manually just to see what they're approving.
func (inst *botInstance) sendOrderReviewCard(chatID int64, o model.BotOrder) {
	plan, _ := botSvc.GetPlan(inst.ownerId, o.PlanId)
	planName := kindLabel(langFA, o.Kind)
	if plan != nil {
		planName = plan.Name
	}
	summary := fmt.Sprintf("#%d — @%s\n%s\n%s\nProof: %s", o.Id, o.BuyerUsername, planName,
		t(langFA, "invoiceAmount", o.FinalAmountToman), orderProofSummary(o))
	if o.TransactionRef != "" {
		summary += "\n" + t(langFA, "invoiceTxRef", o.TransactionRef)
	}
	markup := tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t(langFA, "approve")).WithCallbackData("a:"+strconv.Itoa(o.Id)),
		tu.InlineKeyboardButton(t(langFA, "reject")).WithCallbackData("r:"+strconv.Itoa(o.Id)),
	))
	if o.ProofTgFileId != "" {
		_, _ = inst.bot.SendPhoto(context.Background(), &telego.SendPhotoParams{
			ChatID: tu.ID(chatID), Photo: telego.InputFile{FileID: o.ProofTgFileId}, Caption: summary, ReplyMarkup: markup,
		})
		return
	}
	inst.send(chatID, summary, markup)
}

func orderProofSummary(o model.BotOrder) string {
	if o.ProofText != "" {
		return o.ProofText
	}
	if o.ProofTgFileId != "" {
		return "(photo)"
	}
	return "(none)"
}

func (inst *botInstance) handleCallback(_ *th.Context, query telego.CallbackQuery) error {
	_ = inst.bot.AnswerCallbackQuery(context.Background(), &telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
	if query.Message == nil {
		return nil
	}
	chatID := query.Message.GetChat().ID
	buyerID, buyerUsername := query.From.ID, query.From.Username
	parts := strings.SplitN(query.Data, ":", 3)
	if len(parts) < 2 {
		return nil
	}
	lang := inst.customerLang(buyerID, buyerUsername)

	switch parts[0] {
	case "lang":
		inst.handleSelectLanguage(chatID, buyerID, buyerUsername, parts[1])
	case "buy":
		inst.handleBuyPlan(chatID, parts[1], lang)
	case "email":
		if len(parts) < 3 || parts[1] != "random" {
			return nil
		}
		inst.handleRandomUsername(chatID, buyerID, buyerUsername, parts[2], lang)
	case "inv":
		if len(parts) < 3 {
			return nil
		}
		switch parts[1] {
		case "view":
			if id, err := strconv.Atoi(parts[2]); err == nil {
				inst.sendInvoiceCard(chatID, id, lang)
			}
		case "pay":
			inst.handleInvoicePayStart(chatID, parts[2], lang)
		case "disc":
			inst.handleInvoiceDiscountPrompt(chatID, parts[2], lang)
		case "cancel":
			inst.handleInvoiceCancel(chatID, parts[2], lang)
		}
	case "disc":
		if len(parts) < 3 || parts[1] != "skip" {
			return nil
		}
		inst.handleDiscountCode(chatID, parts[2], "", lang)
	case "pm":
		if len(parts) < 3 {
			return nil
		}
		inst.handleChoosePaymentMethod(chatID, parts[1], parts[2], lang)
	case "pmwallet":
		inst.handlePayFromWallet(chatID, parts[1], lang)
	case "sub":
		if len(parts) < 3 {
			return nil
		}
		switch parts[1] {
		case "view":
			if id, err := strconv.Atoi(parts[2]); err == nil {
				inst.sendSubscriptionDetail(chatID, id, lang)
			}
		case "configs":
			inst.handleGetConfigs(chatID, parts[2], lang)
		case "revoke":
			inst.handleRevokePrompt(chatID, parts[2], lang)
		case "revokeconfirm":
			inst.handleRevokeConfirm(chatID, parts[2], lang)
		case "renew":
			inst.handleRenew(chatID, buyerID, buyerUsername, parts[2], lang)
		case "autorenew":
			inst.handleToggleAutoRenew(chatID, parts[2], lang)
		}
	case "acct":
		switch parts[1] {
		case "topup":
			inst.handleTopupPrompt(chatID, lang)
		case "redeem":
			inst.handleRedeemPrompt(chatID, lang)
		case "notif":
			inst.handleToggleNotifications(chatID, buyerID, buyerUsername, lang)
		case "referral":
			inst.sendReferralInfo(chatID, buyerID, buyerUsername, lang)
		}
	case "a":
		inst.handleApprove(chatID, parts[1])
	case "r":
		inst.handleReject(chatID, parts[1])
	}
	return nil
}

func (inst *botInstance) handleApprove(chatID int64, orderIDStr string) {
	if !isOwnerChat(inst.ownerId, chatID) {
		return
	}
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		return
	}
	order, email, err := botSvc.ApproveOrder(orderID, inst.ownerId)
	if err != nil {
		inst.send(chatID, t(langFA, "approveFailed")+": "+err.Error())
		return
	}
	inst.send(chatID, t(langFA, "approved"))
	inst.notifyBuyerApproved(order, email)
}

func (inst *botInstance) handleReject(chatID int64, orderIDStr string) {
	if !isOwnerChat(inst.ownerId, chatID) {
		return
	}
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		return
	}
	order, err := botSvc.RejectOrder(orderID, inst.ownerId, "")
	if err != nil {
		inst.send(chatID, t(langFA, "rejectFailed"))
		return
	}
	inst.send(chatID, t(langFA, "rejected"))
	inst.notifyBuyerRejected(order)
}

// handleMessage dispatches a plain text/photo message by the buyer's
// in-flight multi-step state (proof/discount/top-up/redeem), falling back to
// the persistent main-menu button text when there's no state in progress.
func (inst *botInstance) handleMessage(_ *th.Context, message telego.Message) error {
	if message.From == nil {
		return nil
	}
	lang := inst.customerLang(message.From.ID, message.From.Username)
	state, ok := inst.states.get(message.Chat.ID)
	if ok {
		switch {
		case strings.HasPrefix(state, "awaiting_proof:"):
			inst.handleProofMessage(message, strings.TrimPrefix(state, "awaiting_proof:"), lang)
			return nil
		case strings.HasPrefix(state, "awaiting_discount:"):
			inst.handleDiscountCode(message.Chat.ID, strings.TrimPrefix(state, "awaiting_discount:"), strings.TrimSpace(message.Text), lang)
			return nil
		case strings.HasPrefix(state, "awaiting_username:"):
			inst.handleUsernameInput(message, strings.TrimPrefix(state, "awaiting_username:"), lang)
			return nil
		case state == "awaiting_topup_amount":
			inst.handleTopupAmount(message, lang)
			return nil
		case state == "awaiting_redeem_code":
			inst.handleRedeemCode(message, lang)
			return nil
		}
	}
	if isOwnerChat(inst.ownerId, message.Chat.ID) {
		inst.handleAdminMenuText(message)
		return nil
	}
	inst.handleMenuText(message, lang)
	return nil
}
