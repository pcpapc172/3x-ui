package resellerbot

import (
	"fmt"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// adminMenuKeyboard is the owner's own management menu — shown instead of
// the buyer shop menu in the owner's linked chat, so managing the shop
// doesn't require falling back to /orders and the web panel for everything.
func adminMenuKeyboard() *telego.ReplyKeyboardMarkup {
	kb := tu.Keyboard(
		tu.KeyboardRow(tu.KeyboardButton(t(langFA, "adminMenuOrders"))),
		tu.KeyboardRow(tu.KeyboardButton(t(langFA, "adminMenuCustomers")), tu.KeyboardButton(t(langFA, "adminMenuStats"))),
	)
	kb.ResizeKeyboard = true
	return kb
}

func (inst *botInstance) sendAdminMenu(chatID int64) {
	inst.send(chatID, t(langFA, "adminWelcome"), adminMenuKeyboard())
}

func adminMenuAction(text string) string {
	switch text {
	case t(langFA, "adminMenuOrders"):
		return "orders"
	case t(langFA, "adminMenuCustomers"):
		return "customers"
	case t(langFA, "adminMenuStats"):
		return "stats"
	}
	return ""
}

func (inst *botInstance) handleAdminMenuText(message telego.Message) {
	switch adminMenuAction(message.Text) {
	case "orders":
		inst.sendPendingOrders(message.Chat.ID)
	case "customers":
		inst.sendCustomersSummary(message.Chat.ID)
	case "stats":
		inst.sendOwnerStats(message.Chat.ID)
	default:
		inst.sendAdminMenu(message.Chat.ID)
	}
}

func (inst *botInstance) sendPendingOrders(chatID int64) {
	orders, err := botSvc.ListOrders(inst.ownerId, model.BotOrderPendingReview)
	if err != nil || len(orders) == 0 {
		inst.send(chatID, t(langFA, "noPendingOrders"))
		return
	}
	for _, o := range orders {
		inst.sendOrderReviewCard(chatID, o)
	}
}

func (inst *botInstance) sendCustomersSummary(chatID int64) {
	customers, err := botSvc.ListCustomers(inst.ownerId)
	if err != nil {
		inst.send(chatID, t(langFA, "error"))
		return
	}
	if len(customers) == 0 {
		inst.send(chatID, t(langFA, "noCustomers"))
		return
	}
	text := t(langFA, "customersCount", len(customers))
	limit := min(len(customers), 20)
	for _, c := range customers[:limit] {
		label := c.BuyerUsername
		if label == "" {
			label = strconv.FormatInt(c.BuyerTgId, 10)
		}
		text += fmt.Sprintf("\n@%s — %d Toman", label, c.WalletBalanceToman)
	}
	inst.send(chatID, text)
}

func (inst *botInstance) sendOwnerStats(chatID int64) {
	stats, err := botSvc.GetOwnerStats(inst.ownerId)
	if err != nil {
		inst.send(chatID, t(langFA, "error"))
		return
	}
	text := fmt.Sprintf("%s\n%s\n%s\n%s",
		t(langFA, "statsCustomers", stats.CustomerCount),
		t(langFA, "statsApprovedOrders", stats.ApprovedOrders),
		t(langFA, "statsPendingReview", stats.PendingReview),
		t(langFA, "statsRevenue", stats.RevenueToman),
	)
	inst.send(chatID, text)
}
