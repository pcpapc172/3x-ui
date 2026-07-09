package controller

import (
	"net/http"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"

	"github.com/gin-gonic/gin"
)

type ResellerController struct {
	resellerService panel.ResellerService
}

func NewResellerController(g *gin.RouterGroup) *ResellerController {
	a := &ResellerController{}
	a.initRouter(g)
	return a
}

func (a *ResellerController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/resellers")
	g.Use(a.requireAdmin)

	g.GET("/list", a.list)
	g.POST("/add", a.add)
	g.POST("/update/:id", a.update)
	g.POST("/del/:id", a.del)
	g.POST("/resetUsage/:id", a.resetUsage)
	g.POST("/toggleEnable/:id", a.toggleEnable)
}

func (a *ResellerController) requireAdmin(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user == nil || user.Role != "admin" {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	c.Next()
}

func (a *ResellerController) list(c *gin.Context) {
	resellers, err := a.resellerService.ListResellers()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, resellers, nil)
}

type addResellerForm struct {
	Username            string  `form:"username" json:"username"`
	Password            string  `form:"password" json:"password"`
	UsageLimit          float64 `form:"usageLimit" json:"usageLimit"`
	AllowedInboundsMode string  `form:"allowedInboundsMode" json:"allowedInboundsMode"`
	AllowedInboundIds   []int   `form:"allowedInboundIds" json:"allowedInboundIds"`
}

func (a *ResellerController) add(c *gin.Context) {
	var form addResellerForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	user, err := a.resellerService.CreateReseller(form.Username, form.Password, int64(form.UsageLimit), form.AllowedInboundsMode, form.AllowedInboundIds)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, user, nil)
}

type updateResellerForm struct {
	Username            string  `form:"username" json:"username"`
	Password            string  `form:"password" json:"password"`
	UsageLimit          float64 `form:"usageLimit" json:"usageLimit"`
	AllowedInboundsMode string  `form:"allowedInboundsMode" json:"allowedInboundsMode"`
	AllowedInboundIds   []int   `form:"allowedInboundIds" json:"allowedInboundIds"`
}

func (a *ResellerController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var form updateResellerForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.resellerService.UpdateReseller(id, form.Username, form.Password, int64(form.UsageLimit), form.AllowedInboundsMode, form.AllowedInboundIds); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, "success", nil)
}

func (a *ResellerController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	deleteClients := c.Query("deleteClients") == "true"
	if err := a.resellerService.DeleteReseller(id, deleteClients); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, "success", nil)
}

func (a *ResellerController) resetUsage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	affected, err := a.resellerService.ResetResellerUsage(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	websocket.BroadcastInvalidate(websocket.MessageTypeInbounds)
	jsonObj(c, gin.H{"reEnabled": affected}, nil)
}

func (a *ResellerController) toggleEnable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	enabled, err := a.resellerService.ToggleEnableReseller(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"enabled": enabled}, nil)
}
