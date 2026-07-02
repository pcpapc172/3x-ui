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
	Username   string `json:"username"`
	Password   string `json:"password"`
	UsageLimit int64  `json:"usageLimit"`
}

func (a *ResellerController) add(c *gin.Context) {
	var form addResellerForm
	if err := c.ShouldBindJSON(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	user, err := a.resellerService.CreateReseller(form.Username, form.Password, form.UsageLimit)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, user, nil)
}

type updateResellerForm struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	UsageLimit int64  `json:"usageLimit"`
}

func (a *ResellerController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var form updateResellerForm
	if err := c.ShouldBindJSON(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.resellerService.UpdateReseller(id, form.Username, form.Password, form.UsageLimit); err != nil {
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
	if err := a.resellerService.DeleteReseller(id); err != nil {
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
