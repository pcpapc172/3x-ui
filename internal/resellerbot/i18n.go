package resellerbot

import "fmt"

// Chat strings for the buyer-facing shop bot are Persian/English only,
// selected per BotCustomer.Language — not the panel's 13-locale
// internal/web/translation system. That system backs a single global
// LocalizerBot fixed at process startup (internal/web/locale), which cannot
// serve independent per-buyer language choice across many owners' bots, so
// it isn't a fit here. See internal/resellerbot's package doc for context.
const (
	langFA = "fa"
	langEN = "en"
)

// t looks up key in lang's table (falling back to Persian, then to the key
// itself if truly missing) and formats it against args like fmt.Sprintf.
func t(lang, key string, args ...any) string {
	if lang != langEN {
		lang = langFA
	}
	table := catalog[lang]
	msg, ok := table[key]
	if !ok {
		msg, ok = catalog[langFA][key]
	}
	if !ok {
		return key
	}
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}

var catalog = map[string]map[string]string{
	langFA: {
		"chooseLanguage": "لطفاً زبان خود را انتخاب کنید:",
		"welcome":        "لطفاً یکی از گزینه‌های زیر را انتخاب کنید:",
		"menuBuyNew":     "🛍 خرید اشتراک جدید",
		"menuTrial":      "🆓 دریافت اشتراک تست",
		"menuInvoices":   "📄 فاکتورهای من",
		"menuSubs":       "💎 اشتراک‌های من",
		"menuAccount":    "👤 حساب کاربری",
		"menuSupport":    "📞 پشتیبانی",
		"menuTutorial":   "📱 آموزش اتصال",
		"btnBack":        "↩️ بازگشت",
		"error":          "خطایی رخ داد. لطفاً دوباره تلاش کنید.",
		"noPlans":        "در حال حاضر پلنی برای فروش موجود نیست.",
		"choosePlan":     "یکی از پلن‌های زیر را انتخاب کنید:",

		"noPaymentMethods": "روش پرداختی تنظیم نشده است.",
		"choosePayment":    "روش پرداخت را انتخاب کنید:",
		"payWithWallet":    "💳 پرداخت از کیف پول (موجودی: %s تومان)",
		"payInstructions":  "لطفاً مبلغ را به شماره زیر واریز کنید:",
		"sendProof":        "پس از پرداخت، رسید یا کد پیگیری تراکنش را ارسال کنید.",
		"proofReceived":    "رسید دریافت شد و برای بررسی ارسال گردید.",
		"askDiscountCode":  "کد تخفیف دارید؟ آن را ارسال کنید یا «رد شدن» را بزنید.",
		"skipDiscount":     "⏭ رد شدن",
		"invalidDiscount":  "کد تخفیف نامعتبر است: %s",
		"discountApplied":  "کد تخفیف اعمال شد: %d تومان تخفیف.",

		"invoiceTitle":         "🧾 فاکتور شما با موفقیت ایجاد شد.",
		"invoiceId":            "🆔 شناسه: %s",
		"invoiceStatus":        "وضعیت: %s",
		"invoiceAmount":        "💰 مبلغ: %d تومان",
		"invoiceDiscount":      "🏷 پس از تخفیف: %s",
		"invoiceKind":          "📦 نوع: %s",
		"invoiceCreatedAt":     "📅 تاریخ ثبت: %s",
		"invoicePaidAt":        "✅ تاریخ پرداخت: %s",
		"invoiceTxRef":         "🔢 شماره تراکنش: %s",
		"invoicePlan":          "📦 اشتراک: %s",
		"btnPay":               "💳 پرداخت فاکتور",
		"btnApplyDiscount":     "🎟 اعمال کد تخفیف",
		"btnCancel":            "❌ لغو فاکتور",
		"orderCanceled":        "فاکتور لغو شد.",
		"orderApproved":        "✅ پرداخت شما تایید شد.\nایمیل کلاینت: %s",
		"orderRejected":        "❌ پرداخت شما رد شد.",
		"insufficientBalance":  "موجودی کیف پول کافی نیست.",
		"statusPendingPayment": "⏳ در انتظار پرداخت",
		"statusPendingReview":  "🔎 در حال بررسی",
		"statusApproved":       "✅ تایید شده",
		"statusRejected":       "🚫 رد شده",
		"statusExpired":        "⌛️ منقضی شده",
		"statusCanceled":       "❌ لغو شده",
		"kindPurchase":         "اشتراک جدید",
		"kindRenewal":          "تمدید اشتراک",
		"kindTopup":            "افزایش موجودی کیف پول",
		"none":                 "—",

		"walletTopupPrompt":  "مبلغ افزایش موجودی را به تومان ارسال کنید:",
		"walletTopupInvalid": "لطفاً یک عدد معتبر ارسال کنید.",
		"redeemPrompt":       "کد هدیه را ارسال کنید:",
		"redeemSuccess":      "✅ %d تومان به کیف پول شما اضافه شد.",
		"redeemFailed":       "کد هدیه نامعتبر است: %s",

		"subListEmpty":           "شما هیچ اشتراکی ندارید.",
		"subUsed":                "مصرف‌شده: %s",
		"subRemaining":           "زمان باقی‌مانده: %s",
		"subLink":                "لینک اشتراک:\n%s",
		"btnGetConfigs":          "📥 دریافت کانفیگ‌ها",
		"btnRevoke":              "♻️ ابطال لینک و قطع اتصال دیگران",
		"btnRenew":               "🛒 تمدید",
		"autoRenewOn":            "🔄 تمدید خودکار: فعال",
		"autoRenewOff":           "🔄 تمدید خودکار: غیرفعال",
		"revokeConfirm":          "با این کار لینک قبلی از کار می‌افتد و همه دستگاه‌های دیگر قطع می‌شوند. ادامه می‌دهید؟",
		"revokeDone":             "✅ لینک جدید صادر شد.",
		"autoRenewToggled":       "تنظیمات تمدید خودکار بروزرسانی شد.",
		"autoRenewSucceeded":     "🔄 اشتراک شما به‌صورت خودکار تمدید شد.",
		"autoRenewFailedBalance": "⚠️ تمدید خودکار اشتراک شما به دلیل کمبود موجودی کیف پول ناموفق بود. لطفاً موجودی خود را افزایش دهید.",

		"accountId":        "🆔 شناسه کاربری: %d",
		"accountBalance":   "💰 موجودی کیف پول: %d تومان",
		"accountReferrals": "👥 تعداد زیرمجموعه‌ها: %d",
		"accountJoined":    "📅 تاریخ عضویت: %s",
		"btnTopup":         "➕ افزایش موجودی",
		"btnRedeem":        "🎁 کد هدیه",
		"btnReferral":      "👥 زیرمجموعه‌گیری",
		"notificationsOn":  "🔔 وضعیت اعلان‌ها: فعال",
		"notificationsOff": "🔕 وضعیت اعلان‌ها: غیرفعال",
		"referralInfo":     "لینک دعوت شما:\n%s\n\nتعداد دعوت‌شدگان: %d\nپاداش دریافتی: %d تومان",

		"trialAlreadyUsed": "شما قبلاً از اشتراک تست استفاده کرده‌اید.",
		"noTrialPlan":      "در حال حاضر اشتراک تست فعال نیست.",

		"newOrderPendingReview": "🆕 فاکتور جدید در انتظار بررسی",
		"approve":               "✅ تایید",
		"reject":                "❌ رد",
		"approved":              "تایید شد.",
		"rejected":              "رد شد.",
		"approveFailed":         "تایید ناموفق بود",
		"rejectFailed":          "رد کردن ناموفق بود.",
		"linkUsage":             "استفاده: /link <code>",
		"linkFailed":            "کد نامعتبر است.",
		"linkSuccess":           "این چت به عنوان مرجع بررسی سفارش‌ها ثبت شد.",
		"noPendingOrders":       "سفارش در انتظار بررسی وجود ندارد.",
	},
	langEN: {
		"chooseLanguage": "Please choose your language:",
		"welcome":        "Please choose one of the options below:",
		"menuBuyNew":     "🛍 Buy new subscription",
		"menuTrial":      "🆓 Get free trial",
		"menuInvoices":   "📄 My invoices",
		"menuSubs":       "💎 My subscriptions",
		"menuAccount":    "👤 My account",
		"menuSupport":    "📞 Support",
		"menuTutorial":   "📱 Connection guide",
		"btnBack":        "↩️ Back",
		"error":          "Something went wrong. Please try again.",
		"noPlans":        "No plans are available right now.",
		"choosePlan":     "Choose one of the plans below:",

		"noPaymentMethods": "No payment methods are configured.",
		"choosePayment":    "Choose a payment method:",
		"payWithWallet":    "💳 Pay from wallet (balance: %s Toman)",
		"payInstructions":  "Please transfer the amount to:",
		"sendProof":        "After paying, send the receipt or transaction reference.",
		"proofReceived":    "Receipt received and sent for review.",
		"askDiscountCode":  "Have a discount code? Send it, or tap Skip.",
		"skipDiscount":     "⏭ Skip",
		"invalidDiscount":  "Invalid discount code: %s",
		"discountApplied":  "Discount applied: %d Toman off.",

		"invoiceTitle":         "🧾 Your invoice was created.",
		"invoiceId":            "🆔 ID: %s",
		"invoiceStatus":        "Status: %s",
		"invoiceAmount":        "💰 Amount: %d Toman",
		"invoiceDiscount":      "🏷 After discount: %s",
		"invoiceKind":          "📦 Type: %s",
		"invoiceCreatedAt":     "📅 Created: %s",
		"invoicePaidAt":        "✅ Paid: %s",
		"invoiceTxRef":         "🔢 Transaction ref: %s",
		"invoicePlan":          "📦 Plan: %s",
		"btnPay":               "💳 Pay invoice",
		"btnApplyDiscount":     "🎟 Apply discount code",
		"btnCancel":            "❌ Cancel invoice",
		"orderCanceled":        "Invoice canceled.",
		"orderApproved":        "✅ Your payment was approved.\nClient email: %s",
		"orderRejected":        "❌ Your payment was rejected.",
		"insufficientBalance":  "Insufficient wallet balance.",
		"statusPendingPayment": "⏳ Awaiting payment",
		"statusPendingReview":  "🔎 Under review",
		"statusApproved":       "✅ Approved",
		"statusRejected":       "🚫 Rejected",
		"statusExpired":        "⌛️ Expired",
		"statusCanceled":       "❌ Canceled",
		"kindPurchase":         "New subscription",
		"kindRenewal":          "Renewal",
		"kindTopup":            "Wallet top-up",
		"none":                 "—",

		"walletTopupPrompt":  "Send the top-up amount in Toman:",
		"walletTopupInvalid": "Please send a valid number.",
		"redeemPrompt":       "Send your gift code:",
		"redeemSuccess":      "✅ %d Toman was added to your wallet.",
		"redeemFailed":       "Invalid gift code: %s",

		"subListEmpty":           "You have no subscriptions yet.",
		"subUsed":                "Used: %s",
		"subRemaining":           "Time left: %s",
		"subLink":                "Subscription link:\n%s",
		"btnGetConfigs":          "📥 Get configs",
		"btnRevoke":              "♻️ Revoke link & disconnect others",
		"btnRenew":               "🛒 Renew",
		"autoRenewOn":            "🔄 Auto-renew: on",
		"autoRenewOff":           "🔄 Auto-renew: off",
		"revokeConfirm":          "This invalidates the old link and disconnects every other device. Continue?",
		"revokeDone":             "✅ A new link was issued.",
		"autoRenewToggled":       "Auto-renew setting updated.",
		"autoRenewSucceeded":     "🔄 Your subscription was renewed automatically.",
		"autoRenewFailedBalance": "⚠️ Auto-renew failed: insufficient wallet balance. Please top up.",

		"accountId":        "🆔 User ID: %d",
		"accountBalance":   "💰 Wallet balance: %d Toman",
		"accountReferrals": "👥 Referrals: %d",
		"accountJoined":    "📅 Joined: %s",
		"btnTopup":         "➕ Top up balance",
		"btnRedeem":        "🎁 Gift code",
		"btnReferral":      "👥 Referrals",
		"notificationsOn":  "🔔 Notifications: on",
		"notificationsOff": "🔕 Notifications: off",
		"referralInfo":     "Your invite link:\n%s\n\nReferred: %d\nEarned: %d Toman",

		"trialAlreadyUsed": "You've already used your free trial.",
		"noTrialPlan":      "No free trial is available right now.",

		"newOrderPendingReview": "🆕 New invoice pending review",
		"approve":               "✅ Approve",
		"reject":                "❌ Reject",
		"approved":              "Approved.",
		"rejected":              "Rejected.",
		"approveFailed":         "Approval failed",
		"rejectFailed":          "Rejection failed.",
		"linkUsage":             "Usage: /link <code>",
		"linkFailed":            "Invalid code.",
		"linkSuccess":           "This chat is now registered to review orders.",
		"noPendingOrders":       "No orders pending review.",
	},
}
