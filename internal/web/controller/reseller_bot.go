package controller

import (
	"io"
	"net/http"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/resellerbot"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

// ResellerBotController exposes self-service Telegram-sales-bot management,
// scoped to the logged-in user (reseller or admin) — not admin-only, unlike
// ResellerController which manages OTHER users' reseller accounts.
type ResellerBotController struct {
	botService panel.BotService
}

func NewResellerBotController(g *gin.RouterGroup) *ResellerBotController {
	a := &ResellerBotController{}
	a.initRouter(g)
	return a
}

func (a *ResellerBotController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/bot")
	g.Use(func(c *gin.Context) {
		enabled, err := (&service.SettingService{}).GetResellerBotEnabled()
		if err != nil || !enabled {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		c.Next()
	})

	g.GET("/config", a.getConfig)
	g.POST("/config", a.saveConfig)
	g.POST("/config/link-code", a.generateLinkCode)

	g.GET("/plans/list", a.listPlans)
	g.POST("/plans/set", a.setPlan)
	g.POST("/plans/del/:id", a.delPlan)

	g.GET("/payment-methods/list", a.listPaymentMethods)
	g.POST("/payment-methods/set", a.setPaymentMethod)
	g.POST("/payment-methods/del/:id", a.delPaymentMethod)

	g.GET("/orders/list", a.listOrders)
	g.POST("/orders/approve/:id", a.approveOrder)
	g.POST("/orders/reject/:id", a.rejectOrder)
	g.GET("/orders/:id/proof-photo", a.orderProofPhoto)

	g.GET("/discount-codes/list", a.listDiscountCodes)
	g.POST("/discount-codes/set", a.setDiscountCode)
	g.POST("/discount-codes/del/:id", a.delDiscountCode)

	g.GET("/redeem-codes/list", a.listRedeemCodes)
	g.POST("/redeem-codes/set", a.setRedeemCode)
	g.POST("/redeem-codes/del/:id", a.delRedeemCode)

	g.GET("/customers/list", a.listCustomers)
	g.POST("/customers/:id/adjust-wallet", a.adjustCustomerWallet)
}

func (a *ResellerBotController) getConfig(c *gin.Context) {
	user := session.GetLoginUser(c)
	cfg, err := a.botService.GetConfig(user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{
		"enabled":         cfg.Enabled,
		"botUsername":     cfg.BotUsername,
		"welcomeMessage":  cfg.WelcomeMessage,
		"linked":          cfg.OwnerChatId != 0,
		"hasToken":        cfg.Token != "",
		"walletEnabled":   cfg.WalletEnabled,
		"referralEnabled": cfg.ReferralEnabled,
		"referralKind":    cfg.ReferralKind,
		"referralValue":   cfg.ReferralValue,
		"referralTrigger": cfg.ReferralTrigger,
		"supportText":     cfg.SupportText,
		"tutorialText":    cfg.TutorialText,
	}, nil)
}

type saveBotConfigForm struct {
	Token           string `form:"token" json:"token"`
	Enabled         bool   `form:"enabled" json:"enabled"`
	WelcomeMessage  string `form:"welcomeMessage" json:"welcomeMessage"`
	WalletEnabled   bool   `form:"walletEnabled" json:"walletEnabled"`
	ReferralEnabled bool   `form:"referralEnabled" json:"referralEnabled"`
	ReferralKind    string `form:"referralKind" json:"referralKind"`
	ReferralValue   int64  `form:"referralValue" json:"referralValue"`
	ReferralTrigger string `form:"referralTrigger" json:"referralTrigger"`
	SupportText     string `form:"supportText" json:"supportText"`
	TutorialText    string `form:"tutorialText" json:"tutorialText"`
}

func (a *ResellerBotController) saveConfig(c *gin.Context) {
	user := session.GetLoginUser(c)
	var form saveBotConfigForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if _, err := a.botService.SaveConfig(user.Id, panel.BotConfigInput{
		Token:           form.Token,
		Enabled:         form.Enabled,
		WelcomeMessage:  form.WelcomeMessage,
		WalletEnabled:   form.WalletEnabled,
		ReferralEnabled: form.ReferralEnabled,
		ReferralKind:    form.ReferralKind,
		ReferralValue:   form.ReferralValue,
		ReferralTrigger: form.ReferralTrigger,
		SupportText:     form.SupportText,
		TutorialText:    form.TutorialText,
	}); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := resellerbot.GetManager().Ensure(user.Id); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, "success", nil)
}

func (a *ResellerBotController) generateLinkCode(c *gin.Context) {
	user := session.GetLoginUser(c)
	code, err := a.botService.GenerateLinkCode(user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"code": code}, nil)
}

func (a *ResellerBotController) listPlans(c *gin.Context) {
	user := session.GetLoginUser(c)
	plans, err := a.botService.ListPlans(user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, plans, nil)
}

type setPlanForm struct {
	Id           int    `form:"id" json:"id"`
	Name         string `form:"name" json:"name"`
	TrafficGB    int64  `form:"trafficGB" json:"trafficGB"`
	DurationDays int    `form:"durationDays" json:"durationDays"`
	PriceToman   int64  `form:"priceToman" json:"priceToman"`
	InboundIds   []int  `form:"inboundIds" json:"inboundIds"`
	Enabled      bool   `form:"enabled" json:"enabled"`
	IsTrial      bool   `form:"isTrial" json:"isTrial"`
}

func (a *ResellerBotController) setPlan(c *gin.Context) {
	user := session.GetLoginUser(c)
	var form setPlanForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	plan := model.BotPlan{
		Id:           form.Id,
		OwnerId:      user.Id,
		Name:         form.Name,
		TrafficGB:    form.TrafficGB,
		DurationDays: form.DurationDays,
		PriceToman:   form.PriceToman,
		InboundIds:   form.InboundIds,
		Enabled:      form.Enabled,
		IsTrial:      form.IsTrial,
	}
	if err := a.botService.SavePlan(&plan); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, "success", nil)
}

func (a *ResellerBotController) delPlan(c *gin.Context) {
	user := session.GetLoginUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.botService.DeletePlan(user.Id, id); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, "success", nil)
}

func (a *ResellerBotController) listPaymentMethods(c *gin.Context) {
	user := session.GetLoginUser(c)
	methods, err := a.botService.ListPaymentMethods(user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, methods, nil)
}

type setPaymentMethodForm struct {
	Id         int    `form:"id" json:"id"`
	Type       string `form:"type" json:"type"`
	Label      string `form:"label" json:"label"`
	Value      string `form:"value" json:"value"`
	HolderName string `form:"holderName" json:"holderName"`
	Enabled    bool   `form:"enabled" json:"enabled"`
}

func (a *ResellerBotController) setPaymentMethod(c *gin.Context) {
	user := session.GetLoginUser(c)
	var form setPaymentMethodForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	method := model.BotPaymentMethod{
		Id:         form.Id,
		OwnerId:    user.Id,
		Type:       form.Type,
		Label:      form.Label,
		Value:      form.Value,
		HolderName: form.HolderName,
		Enabled:    form.Enabled,
	}
	if err := a.botService.SavePaymentMethod(&method); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, "success", nil)
}

func (a *ResellerBotController) delPaymentMethod(c *gin.Context) {
	user := session.GetLoginUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.botService.DeletePaymentMethod(user.Id, id); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, "success", nil)
}

func (a *ResellerBotController) listOrders(c *gin.Context) {
	user := session.GetLoginUser(c)
	status := c.Query("status")
	orders, err := a.botService.ListOrders(user.Id, status)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, orders, nil)
}

// orderId parses the :id param and confirms the order belongs to the
// logged-in user before any handler touches it.
func (a *ResellerBotController) orderId(c *gin.Context, ownerId int) (int, error) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return 0, err
	}
	if _, err := a.botService.GetOrder(ownerId, id); err != nil {
		return 0, err
	}
	return id, nil
}

func (a *ResellerBotController) approveOrder(c *gin.Context) {
	user := session.GetLoginUser(c)
	id, err := a.orderId(c, user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	order, email, err := a.botService.ApproveOrder(id, user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	resellerbot.NotifyOrderApproved(order, email)
	jsonObj(c, gin.H{"clientEmail": email}, nil)
}

type rejectOrderForm struct {
	Reason string `form:"reason" json:"reason"`
}

func (a *ResellerBotController) rejectOrder(c *gin.Context) {
	user := session.GetLoginUser(c)
	id, err := a.orderId(c, user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var form rejectOrderForm
	_ = c.ShouldBind(&form)
	order, err := a.botService.RejectOrder(id, user.Id, form.Reason)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	resellerbot.NotifyOrderRejected(order)
	jsonMsg(c, "success", nil)
}

func (a *ResellerBotController) orderProofPhoto(c *gin.Context) {
	user := session.GetLoginUser(c)
	id, err := a.orderId(c, user.Id)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	order, err := a.botService.GetOrder(user.Id, id)
	if err != nil || order.ProofTgFileId == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	rc, contentType, err := resellerbot.FetchProofPhoto(user.Id, order.ProofTgFileId)
	if err != nil {
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	defer rc.Close()
	c.Header("Content-Type", contentType)
	_, _ = io.Copy(c.Writer, rc)
}

func (a *ResellerBotController) listDiscountCodes(c *gin.Context) {
	user := session.GetLoginUser(c)
	codes, err := a.botService.ListDiscountCodes(user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, codes, nil)
}

type setDiscountCodeForm struct {
	Id        int    `form:"id" json:"id"`
	Code      string `form:"code" json:"code"`
	Kind      string `form:"kind" json:"kind"`
	Value     int64  `form:"value" json:"value"`
	MaxUses   int    `form:"maxUses" json:"maxUses"`
	PlanIds   []int  `form:"planIds" json:"planIds"`
	ExpiresAt int64  `form:"expiresAt" json:"expiresAt"`
	Enabled   bool   `form:"enabled" json:"enabled"`
}

func (a *ResellerBotController) setDiscountCode(c *gin.Context) {
	user := session.GetLoginUser(c)
	var form setDiscountCodeForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	dc := model.BotDiscountCode{
		Id:        form.Id,
		OwnerId:   user.Id,
		Code:      form.Code,
		Kind:      form.Kind,
		Value:     form.Value,
		MaxUses:   form.MaxUses,
		PlanIds:   form.PlanIds,
		ExpiresAt: form.ExpiresAt,
		Enabled:   form.Enabled,
	}
	if err := a.botService.SaveDiscountCode(&dc); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, "success", nil)
}

func (a *ResellerBotController) delDiscountCode(c *gin.Context) {
	user := session.GetLoginUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.botService.DeleteDiscountCode(user.Id, id); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, "success", nil)
}

func (a *ResellerBotController) listRedeemCodes(c *gin.Context) {
	user := session.GetLoginUser(c)
	codes, err := a.botService.ListRedeemCodes(user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, codes, nil)
}

type setRedeemCodeForm struct {
	Id          int    `form:"id" json:"id"`
	Code        string `form:"code" json:"code"`
	AmountToman int64  `form:"amountToman" json:"amountToman"`
	MaxUses     int    `form:"maxUses" json:"maxUses"`
	ExpiresAt   int64  `form:"expiresAt" json:"expiresAt"`
	Enabled     bool   `form:"enabled" json:"enabled"`
}

func (a *ResellerBotController) setRedeemCode(c *gin.Context) {
	user := session.GetLoginUser(c)
	var form setRedeemCodeForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	rc := model.BotRedeemCode{
		Id:          form.Id,
		OwnerId:     user.Id,
		Code:        form.Code,
		AmountToman: form.AmountToman,
		MaxUses:     form.MaxUses,
		ExpiresAt:   form.ExpiresAt,
		Enabled:     form.Enabled,
	}
	if err := a.botService.SaveRedeemCode(&rc); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, "success", nil)
}

func (a *ResellerBotController) delRedeemCode(c *gin.Context) {
	user := session.GetLoginUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.botService.DeleteRedeemCode(user.Id, id); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, "success", nil)
}

func (a *ResellerBotController) listCustomers(c *gin.Context) {
	user := session.GetLoginUser(c)
	customers, err := a.botService.ListCustomers(user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, customers, nil)
}

type adjustWalletForm struct {
	DeltaToman int64  `form:"deltaToman" json:"deltaToman"`
	Note       string `form:"note" json:"note"`
}

func (a *ResellerBotController) adjustCustomerWallet(c *gin.Context) {
	user := session.GetLoginUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var form adjustWalletForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.botService.AdjustWallet(user.Id, id, form.DeltaToman, form.Note); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, "success", nil)
}
