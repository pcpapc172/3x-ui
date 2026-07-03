package controller

import (
	"net/http"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

type MultiplyController struct{}

func NewMultiplyController(g *gin.RouterGroup) *MultiplyController {
	a := &MultiplyController{}
	a.initRouter(g)
	return a
}

func (a *MultiplyController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/multiply")
	g.Use(a.requireAdmin)
	g.GET("/list", a.list)
	g.POST("/set", a.set)
	g.POST("/del/:id", a.del)
}

func (a *MultiplyController) requireAdmin(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user == nil || user.Role != "admin" {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	c.Next()
}

type MultiplyEntry struct {
	Id         int     `json:"id"`
	InboundId  int     `json:"inboundId"`
	InboundTag string  `json:"inboundTag"`
	Rate       float64 `json:"rate"`
}

func (a *MultiplyController) list(c *gin.Context) {
	db := database.GetDB()
	var multipliers []model.InboundMultiplier
	if err := db.Find(&multipliers).Error; err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var inbounds []model.Inbound
	db.Select("id, tag").Find(&inbounds)
	tagById := make(map[int]string, len(inbounds))
	for _, ib := range inbounds {
		tagById[ib.Id] = ib.Tag
	}
	result := make([]MultiplyEntry, 0, len(multipliers))
	for _, m := range multipliers {
		result = append(result, MultiplyEntry{
			Id:         m.Id,
			InboundId:  m.InboundId,
			InboundTag: tagById[m.InboundId],
			Rate:       m.Rate,
		})
	}
	jsonObj(c, result, nil)
}

type setMultiplyForm struct {
	InboundId int     `form:"inboundId" json:"inboundId"`
	Rate      float64 `form:"rate" json:"rate"`
}

func (a *MultiplyController) set(c *gin.Context) {
	var form setMultiplyForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if form.Rate <= 0 {
		form.Rate = 1
	}
	db := database.GetDB()
	var existing model.InboundMultiplier
	result := db.Where("inbound_id = ?", form.InboundId).First(&existing)
	if result.Error == nil {
		existing.Rate = form.Rate
		db.Save(&existing)
	} else {
		m := model.InboundMultiplier{InboundId: form.InboundId, Rate: form.Rate}
		db.Create(&m)
	}
	jsonMsg(c, "success", nil)
}

func (a *MultiplyController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	db := database.GetDB()
	db.Delete(&model.InboundMultiplier{}, id)
	jsonMsg(c, "success", nil)
}
