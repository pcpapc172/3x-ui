package resellerbot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

var botSvc = &panel.BotService{}

const bytesPerGB = 1024 * 1024 * 1024

func t(key string, params ...string) string {
	return locale.I18n(locale.Bot, key, params...)
}

func registerHandlers(h *th.BotHandler, inst *botInstance) {
	h.HandleMessage(inst.handleStart, th.CommandEqual("start"))
	h.HandleMessage(inst.handleOrdersCommand, th.CommandEqual("orders"))
	h.HandleMessage(inst.handleLinkCommand, th.CommandPrefix("link"))
	h.HandleCallbackQuery(inst.handleCallback, th.AnyCallbackQueryWithMessage())
	h.HandleMessage(inst.handleMessage, th.AnyMessage())
}

func (inst *botInstance) send(chatID int64, text string, markup ...telego.ReplyMarkup) {
	params := &telego.SendMessageParams{ChatID: tu.ID(chatID), Text: text, ParseMode: "HTML"}
	if len(markup) > 0 {
		params.ReplyMarkup = markup[0]
	}
	_, _ = inst.bot.SendMessage(context.Background(), params)
}

func (inst *botInstance) handleStart(_ *th.Context, message telego.Message) error {
	inst.states.clear(message.Chat.ID)
	plans, err := botSvc.ListPlans(inst.ownerId)
	if err != nil {
		inst.send(message.Chat.ID, t("botshop.error"))
		return nil
	}
	var rows [][]telego.InlineKeyboardButton
	for _, p := range plans {
		if !p.Enabled {
			continue
		}
		label := fmt.Sprintf("%s — %s / %dd — %d Toman", p.Name, common.FormatTraffic(p.TrafficGB*bytesPerGB), p.DurationDays, p.PriceToman)
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(label).WithCallbackData("p:"+strconv.Itoa(p.Id)),
		))
	}
	if len(rows) == 0 {
		inst.send(message.Chat.ID, t("botshop.noPlans"))
		return nil
	}
	inst.send(message.Chat.ID, t("botshop.welcome"), tu.InlineKeyboard(rows...))
	return nil
}

func (inst *botInstance) handleLinkCommand(_ *th.Context, message telego.Message) error {
	fields := strings.Fields(message.Text)
	if len(fields) < 2 {
		inst.send(message.Chat.ID, t("botshop.linkUsage"))
		return nil
	}
	ok, err := botSvc.VerifyLinkCode(inst.ownerId, fields[1], message.Chat.ID)
	if err != nil || !ok {
		inst.send(message.Chat.ID, t("botshop.linkFailed"))
		return nil
	}
	inst.send(message.Chat.ID, t("botshop.linkSuccess"))
	return nil
}

func (inst *botInstance) handleOrdersCommand(_ *th.Context, message telego.Message) error {
	if !isOwnerChat(inst.ownerId, message.Chat.ID) {
		return nil
	}
	orders, err := botSvc.ListOrders(inst.ownerId, "pending_review")
	if err != nil || len(orders) == 0 {
		inst.send(message.Chat.ID, t("botshop.noPendingOrders"))
		return nil
	}
	for _, o := range orders {
		plan, _ := botSvc.GetPlan(inst.ownerId, o.PlanId)
		summary := fmt.Sprintf("#%d — @%s\n%s\nProof: %s", o.Id, o.BuyerUsername, plan.Name, orderProofSummary(o))
		markup := tu.InlineKeyboard(tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t("botshop.approve")).WithCallbackData("a:"+strconv.Itoa(o.Id)),
			tu.InlineKeyboardButton(t("botshop.reject")).WithCallbackData("r:"+strconv.Itoa(o.Id)),
		))
		if o.ProofTgFileId != "" {
			_, _ = inst.bot.SendPhoto(context.Background(), &telego.SendPhotoParams{
				ChatID: tu.ID(message.Chat.ID), Photo: telego.InputFile{FileID: o.ProofTgFileId}, Caption: summary, ReplyMarkup: markup,
			})
			continue
		}
		inst.send(message.Chat.ID, summary, markup)
	}
	return nil
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
	parts := strings.SplitN(query.Data, ":", 3)
	if len(parts) < 2 {
		return nil
	}

	switch parts[0] {
	case "p":
		inst.handleSelectPlan(chatID, parts[1])
	case "m":
		if len(parts) < 3 {
			return nil
		}
		inst.handleSelectPaymentMethod(chatID, query.From.ID, query.From.Username, parts[1], parts[2])
	case "a":
		inst.handleApprove(chatID, parts[1])
	case "r":
		inst.handleReject(chatID, parts[1])
	}
	return nil
}

func (inst *botInstance) handleSelectPlan(chatID int64, planIDStr string) {
	planID, err := strconv.Atoi(planIDStr)
	if err != nil {
		return
	}
	plan, err := botSvc.GetPlan(inst.ownerId, planID)
	if err != nil {
		inst.send(chatID, t("botshop.error"))
		return
	}
	methods, err := botSvc.ListPaymentMethods(inst.ownerId)
	if err != nil {
		inst.send(chatID, t("botshop.error"))
		return
	}
	var rows [][]telego.InlineKeyboardButton
	for _, m := range methods {
		if !m.Enabled {
			continue
		}
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(m.Label).WithCallbackData(fmt.Sprintf("m:%d:%d", plan.Id, m.Id)),
		))
	}
	if len(rows) == 0 {
		inst.send(chatID, t("botshop.noPaymentMethods"))
		return
	}
	inst.send(chatID, t("botshop.choosePayment"), tu.InlineKeyboard(rows...))
}

func (inst *botInstance) handleSelectPaymentMethod(chatID, buyerTgID int64, buyerUsername, planIDStr, methodIDStr string) {
	planID, err := strconv.Atoi(planIDStr)
	if err != nil {
		return
	}
	methodID, err := strconv.Atoi(methodIDStr)
	if err != nil {
		return
	}
	method, err := botSvc.GetPaymentMethod(inst.ownerId, methodID)
	if err != nil {
		inst.send(chatID, t("botshop.error"))
		return
	}
	order, err := botSvc.CreateOrder(inst.ownerId, planID, buyerTgID, buyerUsername, methodID)
	if err != nil {
		inst.send(chatID, t("botshop.error"))
		return
	}
	instructions := fmt.Sprintf("%s\n%s: %s", t("botshop.payInstructions"), method.Label, method.Value)
	if method.HolderName != "" {
		instructions += "\n" + method.HolderName
	}
	instructions += "\n\n" + t("botshop.sendProof")
	inst.send(chatID, instructions)
	inst.states.set(chatID, "awaiting_proof:"+strconv.Itoa(order.Id))
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
		inst.send(chatID, t("botshop.approveFailed")+": "+err.Error())
		return
	}
	inst.send(chatID, t("botshop.approved"))
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
		inst.send(chatID, t("botshop.rejectFailed"))
		return
	}
	inst.send(chatID, t("botshop.rejected"))
	inst.notifyBuyerRejected(order)
}

func (inst *botInstance) handleMessage(_ *th.Context, message telego.Message) error {
	state, ok := inst.states.get(message.Chat.ID)
	if !ok || !strings.HasPrefix(state, "awaiting_proof:") {
		return nil
	}
	orderID, err := strconv.Atoi(strings.TrimPrefix(state, "awaiting_proof:"))
	if err != nil {
		return nil
	}
	var fileID string
	if len(message.Photo) > 0 {
		fileID = message.Photo[len(message.Photo)-1].FileID
	}
	if err := botSvc.SubmitProof(orderID, message.Text, fileID); err != nil {
		return nil
	}
	inst.states.clear(message.Chat.ID)
	inst.send(message.Chat.ID, t("botshop.proofReceived"))
	inst.notifyOwnerPendingReview(orderID)
	return nil
}
