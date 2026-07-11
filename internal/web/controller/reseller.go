package controller

import (
	"net/http"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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
	g.GET("/:id/clients", a.getClients)
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

func (a *ResellerController) getClients(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	db := database.GetDB()
	var records []model.ClientRecord
	db.Where("owner_id = ?", id).Order("id ASC").Find(&records)

	type clientTraffic struct {
		Up         int64 `json:"up"`
		Down       int64 `json:"down"`
		LastOnline int64 `json:"lastOnline"`
	}
	type clientWithAttachments struct {
		model.ClientRecord
		Traffic    clientTraffic `json:"traffic"`
		InboundIds []int         `json:"inboundIds"`
	}

	if len(records) == 0 {
		jsonObj(c, []clientWithAttachments{}, nil)
		return
	}

	clientIds := make([]int, 0, len(records))
	emails := make([]string, 0, len(records))
	for i := range records {
		clientIds = append(clientIds, records[i].Id)
		if records[i].Email != "" {
			emails = append(emails, records[i].Email)
		}
	}

	type clientInbound struct {
		ClientId  int `gorm:"column:client_id"`
		InboundId int `gorm:"column:inbound_id"`
	}
	attachments := make(map[int][]int, len(records))
	var links []clientInbound
	db.Where("client_id IN ?", clientIds).Find(&links)
	for _, l := range links {
		attachments[l.ClientId] = append(attachments[l.ClientId], l.InboundId)
	}

	type trafficRow struct {
		Email      string `gorm:"column:email"`
		Up         int64  `gorm:"column:up"`
		Down       int64  `gorm:"column:down"`
		LastOnline int64  `gorm:"column:last_online"`
	}
	trafficByEmail := make(map[string]clientTraffic, len(emails))
	if len(emails) > 0 {
		var trafficRows []trafficRow
		db.Table("client_traffics").Where("email IN ?", emails).
			Select("email, up, down, last_online").
			Find(&trafficRows)
		for _, tr := range trafficRows {
			trafficByEmail[tr.Email] = clientTraffic{Up: tr.Up, Down: tr.Down, LastOnline: tr.LastOnline}
		}
	}

	result := make([]clientWithAttachments, 0, len(records))
	for _, r := range records {
		cwt := clientWithAttachments{
			ClientRecord: r,
			Traffic:      trafficByEmail[r.Email],
			InboundIds:   attachments[r.Id],
		}
		result = append(result, cwt)
	}
	jsonObj(c, result, nil)
}

type addResellerForm struct {
	Username            string  `form:"username" json:"username"`
	Password            string  `form:"password" json:"password"`
	UsageLimit          float64 `form:"usageLimit" json:"usageLimit"`
	AllowedInboundsMode string  `form:"allowedInboundsMode" json:"allowedInboundsMode"`
	AllowedInboundIds   []int   `form:"allowedInboundIds" json:"allowedInboundIds"`
	Multiplier          float64 `form:"multiplier" json:"multiplier"`
}

func (a *ResellerController) add(c *gin.Context) {
	var form addResellerForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	user, err := a.resellerService.CreateReseller(form.Username, form.Password, int64(form.UsageLimit), form.AllowedInboundsMode, form.AllowedInboundIds, form.Multiplier)
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
	Multiplier          float64 `form:"multiplier" json:"multiplier"`
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
	if err := a.resellerService.UpdateReseller(id, form.Username, form.Password, int64(form.UsageLimit), form.AllowedInboundsMode, form.AllowedInboundIds, form.Multiplier); err != nil {
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
