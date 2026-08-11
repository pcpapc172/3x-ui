package resellerbot

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/jalali"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const bytesPerGB = 1024 * 1024 * 1024

func jalaliDateTime(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	tm := time.UnixMilli(ms).In(time.Local)
	y, m, d := jalali.ToJalali(tm.Year(), int(tm.Month()), tm.Day())
	return fmt.Sprintf("%02d:%02d - %d/%02d/%02d", tm.Hour(), tm.Minute(), y, m, d)
}

func statusLabel(lang, status string) string {
	switch status {
	case model.BotOrderPendingPayment:
		return t(lang, "statusPendingPayment")
	case model.BotOrderPendingReview:
		return t(lang, "statusPendingReview")
	case model.BotOrderApproved:
		return t(lang, "statusApproved")
	case model.BotOrderRejected:
		return t(lang, "statusRejected")
	case model.BotOrderExpired:
		return t(lang, "statusExpired")
	case model.BotOrderCanceled:
		return t(lang, "statusCanceled")
	}
	return status
}

func kindLabel(lang, kind string) string {
	switch kind {
	case model.BotOrderKindRenewal:
		return t(lang, "kindRenewal")
	case model.BotOrderKindTopup:
		return t(lang, "kindTopup")
	default:
		return t(lang, "kindPurchase")
	}
}

func (inst *botInstance) sendPlanList(chatID int64, lang string) {
	plans, err := botSvc.ListPlans(inst.ownerId)
	if err != nil {
		inst.send(chatID, t(lang, "error"))
		return
	}
	var rows [][]telego.InlineKeyboardButton
	for _, p := range plans {
		if !p.Enabled || p.IsTrial {
			continue
		}
		label := fmt.Sprintf("%s — %s / %dd — %d Toman", p.Name, common.FormatTraffic(p.TrafficGB*bytesPerGB), p.DurationDays, p.PriceToman)
		rows = append(rows, tu.InlineKeyboardRow(tu.InlineKeyboardButton(label).WithCallbackData("buy:"+strconv.Itoa(p.Id))))
	}
	if len(rows) == 0 {
		inst.send(chatID, t(lang, "noPlans"))
		return
	}
	inst.send(chatID, t(lang, "choosePlan"), tu.InlineKeyboard(rows...))
}

func (inst *botInstance) handleBuyPlan(chatID int64, planIDStr, lang string) {
	if _, err := strconv.Atoi(planIDStr); err != nil {
		return
	}
	inst.states.set(chatID, "awaiting_username:"+planIDStr)
	inst.send(chatID, t(lang, "askUsername"), tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t(lang, "usernameRandom")).WithCallbackData("email:random:"+planIDStr),
	)))
}

// validUsername mirrors this codebase's client-email character rule
// (internal/web/service's unexported hasForbiddenClientChar): no path
// separators, control characters, or whitespace.
func validUsername(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if r == '/' || r == '\\' || r < 0x20 || r == 0x7f || unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func (inst *botInstance) createPurchaseAndShowInvoice(chatID, buyerTgID int64, buyerUsername, planIDStr, requestedEmail, lang string) {
	planID, err := strconv.Atoi(planIDStr)
	if err != nil {
		return
	}
	order, err := botSvc.CreatePurchaseOrder(inst.ownerId, planID, buyerTgID, buyerUsername, 0, "", requestedEmail)
	if err != nil {
		inst.send(chatID, t(lang, "error"))
		return
	}
	inst.sendInvoiceCard(chatID, order.Id, lang)
}

func (inst *botInstance) handleRandomUsername(chatID, buyerTgID int64, buyerUsername, planIDStr, lang string) {
	inst.states.clear(chatID)
	inst.createPurchaseAndShowInvoice(chatID, buyerTgID, buyerUsername, planIDStr, "", lang)
}

func (inst *botInstance) handleUsernameInput(message telego.Message, planIDStr, lang string) {
	chatID := message.Chat.ID
	username := strings.TrimSpace(message.Text)
	if !validUsername(username) {
		inst.send(chatID, t(lang, "usernameInvalid"))
		return
	}
	taken, err := botSvc.IsClientEmailTaken(username)
	if err != nil {
		inst.send(chatID, t(lang, "error"))
		return
	}
	if taken {
		inst.send(chatID, t(lang, "usernameTaken"))
		return
	}
	inst.states.clear(chatID)
	inst.createPurchaseAndShowInvoice(chatID, message.From.ID, message.From.Username, planIDStr, username, lang)
}

func (inst *botInstance) handleRenew(chatID, buyerTgID int64, buyerUsername, subIDStr, lang string) {
	subID, err := strconv.Atoi(subIDStr)
	if err != nil {
		return
	}
	order, err := botSvc.CreateRenewalOrder(inst.ownerId, subID, buyerTgID, buyerUsername, 0, "")
	if err != nil {
		inst.send(chatID, t(lang, "error"))
		return
	}
	inst.sendInvoiceCard(chatID, order.Id, lang)
}

func (inst *botInstance) handleTrial(chatID, buyerTgID int64, buyerUsername, lang string) {
	email, err := botSvc.ClaimTrial(inst.ownerId, buyerTgID, buyerUsername)
	if err != nil {
		if err.Error() == "trial already used" {
			inst.send(chatID, t(lang, "trialAlreadyUsed"))
			return
		}
		inst.send(chatID, t(lang, "noTrialPlan"))
		return
	}
	inst.send(chatID, t(lang, "orderApproved", email))
}

// sendInvoiceCard renders an order as the multi-field invoice message the
// reference design shows, with Pay/Apply-discount/Cancel buttons — reused
// for a freshly created order, "my invoices" list taps, and post-discount
// re-renders.
func (inst *botInstance) sendInvoiceCard(chatID int64, orderID int, lang string) {
	order, err := botSvc.GetOrder(inst.ownerId, orderID)
	if err != nil {
		inst.send(chatID, t(lang, "error"))
		return
	}
	plan, _ := botSvc.GetPlan(inst.ownerId, order.PlanId)
	planName := "-"
	if plan != nil {
		planName = plan.Name
	}

	discount := t(lang, "none")
	if order.DiscountAmountToman > 0 {
		discount = fmt.Sprintf("%d Toman", order.DiscountAmountToman)
	}

	lines := []string{
		t(lang, "invoiceTitle"),
		"",
		t(lang, "invoiceId", order.PublicToken),
		t(lang, "invoiceStatus", statusLabel(lang, order.Status)),
		t(lang, "invoiceAmount", order.FinalAmountToman),
		t(lang, "invoiceDiscount", discount),
		t(lang, "invoiceKind", kindLabel(lang, order.Kind)),
		t(lang, "invoiceCreatedAt", jalaliDateTime(order.CreatedAt)),
		t(lang, "invoicePaidAt", jalaliDateTime(order.PaidAt)),
	}
	if order.TransactionRef != "" {
		lines = append(lines, t(lang, "invoiceTxRef", order.TransactionRef))
	}
	if order.Kind != model.BotOrderKindTopup {
		lines = append(lines, t(lang, "invoicePlan", planName))
	}
	text := ""
	for i, l := range lines {
		if i > 0 {
			text += "\n"
		}
		text += l
	}

	var rows [][]telego.InlineKeyboardButton
	if order.Status == model.BotOrderPendingPayment {
		rows = append(rows, tu.InlineKeyboardRow(tu.InlineKeyboardButton(t(lang, "btnPay")).WithCallbackData("inv:pay:"+strconv.Itoa(order.Id))))
		if order.DiscountCodeId == 0 && order.Kind != model.BotOrderKindTopup {
			rows = append(rows, tu.InlineKeyboardRow(tu.InlineKeyboardButton(t(lang, "btnApplyDiscount")).WithCallbackData("inv:disc:"+strconv.Itoa(order.Id))))
		}
		rows = append(rows, tu.InlineKeyboardRow(tu.InlineKeyboardButton(t(lang, "btnCancel")).WithCallbackData("inv:cancel:"+strconv.Itoa(order.Id))))
	}
	if len(rows) > 0 {
		inst.send(chatID, text, tu.InlineKeyboard(rows...))
		return
	}
	inst.send(chatID, text)
}

// sendInvoiceList shows the buyer's own invoices as tappable buttons (each
// reopens the full invoice card via sendInvoiceCard) — the "My invoices" menu.
func (inst *botInstance) sendInvoiceList(chatID, buyerTgID int64, buyerUsername, lang string) {
	orders, err := botSvc.ListOrdersForBuyer(inst.ownerId, buyerTgID)
	if err != nil || len(orders) == 0 {
		inst.send(chatID, t(lang, "subListEmpty"))
		return
	}
	var rows [][]telego.InlineKeyboardButton
	for _, o := range orders {
		label := fmt.Sprintf("#%s — %s", o.PublicToken, statusLabel(lang, o.Status))
		rows = append(rows, tu.InlineKeyboardRow(tu.InlineKeyboardButton(label).WithCallbackData("inv:view:"+strconv.Itoa(o.Id))))
	}
	inst.send(chatID, t(lang, "menuInvoices"), tu.InlineKeyboard(rows...))
}

func (inst *botInstance) handleInvoiceDiscountPrompt(chatID int64, orderIDStr, lang string) {
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		return
	}
	inst.states.set(chatID, "awaiting_discount:"+orderIDStr)
	inst.send(chatID, t(lang, "askDiscountCode"), tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t(lang, "skipDiscount")).WithCallbackData("disc:skip:"+strconv.Itoa(orderID)),
	)))
}

func (inst *botInstance) handleDiscountCode(chatID int64, orderIDStr, code, lang string) {
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		return
	}
	inst.states.clear(chatID)
	if code != "" {
		if _, err := botSvc.ApplyDiscountToOrder(orderID, code); err != nil {
			inst.send(chatID, t(lang, "invalidDiscount", err.Error()))
		}
	}
	inst.sendInvoiceCard(chatID, orderID, lang)
}

func (inst *botInstance) handleInvoiceCancel(chatID int64, orderIDStr, lang string) {
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		return
	}
	if err := botSvc.CancelOrder(inst.ownerId, orderID); err != nil {
		inst.send(chatID, t(lang, "error"))
		return
	}
	inst.send(chatID, t(lang, "orderCanceled"))
}

func (inst *botInstance) handleInvoicePayStart(chatID int64, orderIDStr, lang string) {
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		return
	}
	order, err := botSvc.GetOrder(inst.ownerId, orderID)
	if err != nil {
		inst.send(chatID, t(lang, "error"))
		return
	}
	methods, err := botSvc.ListPaymentMethods(inst.ownerId)
	if err != nil {
		inst.send(chatID, t(lang, "error"))
		return
	}
	var rows [][]telego.InlineKeyboardButton
	cfg, _ := botSvc.GetConfig(inst.ownerId)
	if cfg != nil && cfg.WalletEnabled {
		if customer, cErr := botSvc.GetOrCreateCustomer(inst.ownerId, order.BuyerTgId, order.BuyerUsername); cErr == nil &&
			customer.WalletBalanceToman >= order.FinalAmountToman {
			rows = append(rows, tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t(lang, "payWithWallet", strconv.FormatInt(customer.WalletBalanceToman, 10))).
					WithCallbackData("pmwallet:"+strconv.Itoa(order.Id)),
			))
		}
	}
	for _, m := range methods {
		if !m.Enabled {
			continue
		}
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(m.Label).WithCallbackData(fmt.Sprintf("pm:%d:%d", order.Id, m.Id)),
		))
	}
	if len(rows) == 0 {
		inst.send(chatID, t(lang, "noPaymentMethods"))
		return
	}
	inst.send(chatID, t(lang, "choosePayment"), tu.InlineKeyboard(rows...))
}

func (inst *botInstance) handleChoosePaymentMethod(chatID int64, orderIDStr, methodIDStr, lang string) {
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		return
	}
	methodID, err := strconv.Atoi(methodIDStr)
	if err != nil {
		return
	}
	method, err := botSvc.GetPaymentMethod(inst.ownerId, methodID)
	if err != nil {
		inst.send(chatID, t(lang, "error"))
		return
	}
	if err := botSvc.SetOrderPaymentMethod(orderID, methodID); err != nil {
		inst.send(chatID, t(lang, "error"))
		return
	}
	instructions := t(lang, "payInstructions") + "\n" + method.Label + ": " + method.Value
	if method.HolderName != "" {
		instructions += "\n" + method.HolderName
	}
	instructions += "\n\n" + t(lang, "sendProof")
	inst.send(chatID, instructions)
	inst.states.set(chatID, "awaiting_proof:"+orderIDStr)
}

func (inst *botInstance) handlePayFromWallet(chatID int64, orderIDStr, lang string) {
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		return
	}
	order, email, err := botSvc.PayOrderFromWallet(orderID)
	if err != nil {
		if err.Error() == "insufficient wallet balance" {
			inst.send(chatID, t(lang, "insufficientBalance"))
		} else {
			inst.send(chatID, t(lang, "error")+" ("+err.Error()+")")
		}
		return
	}
	if order.Kind == model.BotOrderKindTopup {
		inst.send(chatID, t(lang, "redeemSuccess", order.FinalAmountToman))
		return
	}
	sub, err := botSvc.GetSubscriptionByEmail(inst.ownerId, email)
	if err != nil {
		inst.send(chatID, t(lang, "orderApproved", email))
		return
	}
	text, kb := subscriptionCard(sub, lang)
	inst.send(chatID, t(lang, "subscriptionActivated")+"\n\n"+text, kb)
}

func (inst *botInstance) handleProofMessage(message telego.Message, orderIDStr, lang string) {
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		return
	}
	var fileID string
	if len(message.Photo) > 0 {
		fileID = message.Photo[len(message.Photo)-1].FileID
	}
	if err := botSvc.SubmitProof(orderID, message.Text, fileID, ""); err != nil {
		return
	}
	inst.states.clear(message.Chat.ID)
	inst.send(message.Chat.ID, t(lang, "proofReceived"))
	inst.notifyOwnerPendingReview(orderID)
}
