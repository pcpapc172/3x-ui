package controller

import (
	"io"
	"net/http"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/resellerbot"
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
}

func (a *ResellerBotController) getConfig(c *gin.Context) {
	user := session.GetLoginUser(c)
	cfg, err := a.botService.GetConfig(user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{
		"enabled":        cfg.Enabled,
		"botUsername":    cfg.BotUsername,
		"welcomeMessage": cfg.WelcomeMessage,
		"linked":         cfg.OwnerChatId != 0,
		"hasToken":       cfg.Token != "",
	}, nil)
}

type saveBotConfigForm struct {
	Token          string `form:"token" json:"token"`
	Enabled        bool   `form:"enabled" json:"enabled"`
	WelcomeMessage string `form:"welcomeMessage" json:"welcomeMessage"`
}

func (a *ResellerBotController) saveConfig(c *gin.Context) {
	user := session.GetLoginUser(c)
	var form saveBotConfigForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if _, err := a.botService.SaveConfig(user.Id, form.Token, form.Enabled, form.WelcomeMessage); err != nil {
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

func (a *ResellerBotController) setPlan(c *gin.Context) {
	user := session.GetLoginUser(c)
	var plan model.BotPlan
	if err := c.ShouldBind(&plan); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	plan.OwnerId = user.Id
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

func (a *ResellerBotController) setPaymentMethod(c *gin.Context) {
	user := session.GetLoginUser(c)
	var method model.BotPaymentMethod
	if err := c.ShouldBind(&method); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	method.OwnerId = user.Id
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
