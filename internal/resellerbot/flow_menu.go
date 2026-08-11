package resellerbot

import (
	"context"
	"strings"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

// mainMenuKeyboard is the persistent bottom keyboard shown after language
// selection, matching the reference layout: buy / trial / invoices / subs /
// account, then support+tutorial paired, then a lone back-to-menu row.
func mainMenuKeyboard(lang string) *telego.ReplyKeyboardMarkup {
	kb := tu.Keyboard(
		tu.KeyboardRow(tu.KeyboardButton(t(lang, "menuBuyNew"))),
		tu.KeyboardRow(tu.KeyboardButton(t(lang, "menuTrial"))),
		tu.KeyboardRow(tu.KeyboardButton(t(lang, "menuInvoices")), tu.KeyboardButton(t(lang, "menuSubs"))),
		tu.KeyboardRow(tu.KeyboardButton(t(lang, "menuAccount"))),
		tu.KeyboardRow(tu.KeyboardButton(t(lang, "menuSupport")), tu.KeyboardButton(t(lang, "menuTutorial"))),
	)
	kb.ResizeKeyboard = true
	return kb
}

func languageKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("فارسی 🇮🇷").WithCallbackData("lang:"+langFA),
		tu.InlineKeyboardButton("English 🇬🇧").WithCallbackData("lang:"+langEN),
	))
}

// handleStart clears any in-flight step, resolves (or creates) the buyer's
// persistent customer profile, links a referrer from a "/start ref_CODE"
// deep-link payload on first contact, and shows the language picker on a
// genuinely first-ever contact or the main menu otherwise.
func (inst *botInstance) handleStart(_ *th.Context, message telego.Message) error {
	if message.From == nil {
		return nil
	}
	inst.states.clear(message.Chat.ID)
	if isOwnerChat(inst.ownerId, message.Chat.ID) {
		inst.sendAdminMenu(message.Chat.ID)
		return nil
	}
	customer, err := botSvc.GetOrCreateCustomer(inst.ownerId, message.From.ID, message.From.Username)
	if err != nil {
		inst.send(message.Chat.ID, t(langFA, "error"))
		return nil
	}

	if payload := strings.TrimSpace(strings.TrimPrefix(message.Text, "/start")); payload != "" {
		if code, ok := strings.CutPrefix(strings.TrimSpace(payload), "ref_"); ok && code != "" {
			if referrer, rErr := botSvc.GetCustomerByReferralCode(inst.ownerId, code); rErr == nil && referrer != nil {
				_ = botSvc.SetReferrer(customer.Id, referrer.Id)
			}
		}
	}

	if customer.Language == "" {
		inst.send(message.Chat.ID, t(langFA, "chooseLanguage")+"\n"+t(langEN, "chooseLanguage"), languageKeyboard())
		return nil
	}
	inst.sendMainMenu(message.Chat.ID, customer.Language)
	return nil
}

func (inst *botInstance) sendMainMenu(chatID int64, lang string) {
	inst.send(chatID, t(lang, "welcome"), mainMenuKeyboard(lang))
}

func (inst *botInstance) handleSelectLanguage(chatID, buyerTgID int64, buyerUsername, lang string) {
	if lang != langFA && lang != langEN {
		return
	}
	customer, err := botSvc.GetOrCreateCustomer(inst.ownerId, buyerTgID, buyerUsername)
	if err != nil {
		return
	}
	_ = botSvc.SetLanguage(customer.Id, lang)
	inst.sendMainMenu(chatID, lang)
}

// menuAction maps an incoming reply-keyboard button's text to an action key,
// checked against the customer's own language (reply-keyboard buttons send
// their label as plain text, so this is a reverse lookup, not a callback).
func menuAction(lang, text string) string {
	switch text {
	case t(lang, "menuBuyNew"):
		return "buy"
	case t(lang, "menuTrial"):
		return "trial"
	case t(lang, "menuInvoices"):
		return "invoices"
	case t(lang, "menuSubs"):
		return "subs"
	case t(lang, "menuAccount"):
		return "account"
	case t(lang, "menuSupport"):
		return "support"
	case t(lang, "menuTutorial"):
		return "tutorial"
	}
	return ""
}

func (inst *botInstance) handleMenuText(message telego.Message, lang string) bool {
	switch menuAction(lang, message.Text) {
	case "buy":
		inst.sendPlanList(message.Chat.ID, lang)
	case "trial":
		inst.handleTrial(message.Chat.ID, message.From.ID, message.From.Username, lang)
	case "invoices":
		inst.sendInvoiceList(message.Chat.ID, message.From.ID, message.From.Username, lang)
	case "subs":
		inst.sendSubscriptionList(message.Chat.ID, message.From.ID, message.From.Username, lang)
	case "account":
		inst.sendAccountCard(message.Chat.ID, message.From.ID, message.From.Username, lang)
	case "support":
		inst.sendConfigText(message.Chat.ID, lang, func() string { cfg, _ := configFor(inst.ownerId); return cfg.SupportText })
	case "tutorial":
		inst.sendConfigText(message.Chat.ID, lang, func() string { cfg, _ := configFor(inst.ownerId); return cfg.TutorialText })
	default:
		return false
	}
	return true
}

func (inst *botInstance) sendConfigText(chatID int64, lang string, get func() string) {
	text := get()
	if text == "" {
		text = t(lang, "error")
	}
	inst.send(chatID, text)
}

func (inst *botInstance) send(chatID int64, text string, markup ...telego.ReplyMarkup) {
	params := &telego.SendMessageParams{ChatID: tu.ID(chatID), Text: text, ParseMode: "HTML"}
	if len(markup) > 0 {
		params.ReplyMarkup = markup[0]
	}
	_, _ = inst.bot.SendMessage(context.Background(), params)
}
