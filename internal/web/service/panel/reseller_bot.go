package panel

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"gorm.io/gorm"
)

const (
	bytesPerGB    = 1024 * 1024 * 1024
	msPerDay      = 24 * 60 * 60 * 1000
	staleOrderTTL = 30 * time.Minute
)

// BotService owns the DB-backed configuration, plans, payment methods and
// order review for one owner's Telegram sales bot. Order creation happens
// here too (called from internal/resellerbot's router) so every DB write for
// this feature lives in one place; the bot process lifecycle itself is
// owned by internal/resellerbot.Manager, not this package.
type BotService struct{}

func (s *BotService) GetConfig(ownerId int) (*model.BotConfig, error) {
	db := database.GetDB()
	cfg := &model.BotConfig{}
	err := db.Where("owner_id = ?", ownerId).First(cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &model.BotConfig{OwnerId: ownerId}, nil
	}
	return cfg, err
}

func (s *BotService) SaveConfig(ownerId int, token string, enabled bool, welcomeMessage string) (*model.BotConfig, error) {
	db := database.GetDB()
	cfg := &model.BotConfig{}
	err := db.Where("owner_id = ?", ownerId).First(cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		cfg = &model.BotConfig{OwnerId: ownerId}
		if err := db.Create(cfg).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	updates := map[string]any{
		"enabled":         enabled,
		"welcome_message": welcomeMessage,
	}
	if token != "" {
		updates["token"] = token
	}
	if err := db.Model(cfg).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetConfig(ownerId)
}

// GenerateLinkCode mints a fresh one-time code the owner sends to their own
// bot as "/link <code>" to register their chat as the order-review target.
func (s *BotService) GenerateLinkCode(ownerId int) (string, error) {
	code := random.Seq(8)
	db := database.GetDB()
	cfg, err := s.GetConfig(ownerId)
	if err != nil {
		return "", err
	}
	if cfg.Id == 0 {
		cfg = &model.BotConfig{OwnerId: ownerId, LinkCode: code}
		if err := db.Create(cfg).Error; err != nil {
			return "", err
		}
		return code, nil
	}
	if err := db.Model(&model.BotConfig{}).Where("owner_id = ?", ownerId).Update("link_code", code).Error; err != nil {
		return "", err
	}
	return code, nil
}

// VerifyLinkCode is called from the owner's own running bot instance when it
// receives "/link <code>". A match registers chatId as the review target and
// consumes the code.
func (s *BotService) VerifyLinkCode(ownerId int, code string, chatId int64) (bool, error) {
	if code == "" {
		return false, nil
	}
	db := database.GetDB()
	result := db.Model(&model.BotConfig{}).
		Where("owner_id = ? AND link_code = ?", ownerId, code).
		Updates(map[string]any{"owner_chat_id": chatId, "link_code": ""})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (s *BotService) ListPlans(ownerId int) ([]model.BotPlan, error) {
	var plans []model.BotPlan
	err := database.GetDB().Where("owner_id = ?", ownerId).Order("sort_order ASC, id ASC").Find(&plans).Error
	return plans, err
}

func (s *BotService) GetPlan(ownerId, id int) (*model.BotPlan, error) {
	plan := &model.BotPlan{}
	err := database.GetDB().Where("id = ? AND owner_id = ?", id, ownerId).First(plan).Error
	return plan, err
}

func (s *BotService) SavePlan(plan *model.BotPlan) error {
	if plan.Name == "" {
		return errors.New("plan name is required")
	}
	if plan.TrafficGB < 0 || plan.DurationDays <= 0 || plan.PriceToman < 0 {
		return errors.New("plan traffic/duration/price out of range")
	}
	if len(plan.InboundIds) == 0 {
		return errors.New("at least one inbound is required")
	}
	db := database.GetDB()
	if plan.Id == 0 {
		return db.Create(plan).Error
	}
	idsJSON, _ := json.Marshal(plan.InboundIds)
	return db.Model(&model.BotPlan{}).Where("id = ? AND owner_id = ?", plan.Id, plan.OwnerId).Updates(map[string]any{
		"name":          plan.Name,
		"traffic_gb":    plan.TrafficGB,
		"duration_days": plan.DurationDays,
		"price_toman":   plan.PriceToman,
		"inbound_ids":   string(idsJSON),
		"enabled":       plan.Enabled,
		"sort_order":    plan.SortOrder,
	}).Error
}

func (s *BotService) DeletePlan(ownerId, id int) error {
	return database.GetDB().Where("id = ? AND owner_id = ?", id, ownerId).Delete(&model.BotPlan{}).Error
}

func (s *BotService) ListPaymentMethods(ownerId int) ([]model.BotPaymentMethod, error) {
	var methods []model.BotPaymentMethod
	err := database.GetDB().Where("owner_id = ?", ownerId).Order("sort_order ASC, id ASC").Find(&methods).Error
	return methods, err
}

func (s *BotService) GetPaymentMethod(ownerId, id int) (*model.BotPaymentMethod, error) {
	method := &model.BotPaymentMethod{}
	err := database.GetDB().Where("id = ? AND owner_id = ?", id, ownerId).First(method).Error
	return method, err
}

func (s *BotService) SavePaymentMethod(method *model.BotPaymentMethod) error {
	if method.Type != "card" && method.Type != "crypto" {
		return errors.New("payment method type must be card or crypto")
	}
	if method.Label == "" || method.Value == "" {
		return errors.New("payment method label and value are required")
	}
	db := database.GetDB()
	if method.Id == 0 {
		return db.Create(method).Error
	}
	return db.Model(&model.BotPaymentMethod{}).Where("id = ? AND owner_id = ?", method.Id, method.OwnerId).Updates(map[string]any{
		"type":        method.Type,
		"label":       method.Label,
		"value":       method.Value,
		"holder_name": method.HolderName,
		"enabled":     method.Enabled,
		"sort_order":  method.SortOrder,
	}).Error
}

func (s *BotService) DeletePaymentMethod(ownerId, id int) error {
	return database.GetDB().Where("id = ? AND owner_id = ?", id, ownerId).Delete(&model.BotPaymentMethod{}).Error
}

func (s *BotService) ListOrders(ownerId int, status string) ([]model.BotOrder, error) {
	q := database.GetDB().Where("owner_id = ?", ownerId)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var orders []model.BotOrder
	err := q.Order("created_at DESC").Find(&orders).Error
	return orders, err
}

func (s *BotService) GetOrder(ownerId, id int) (*model.BotOrder, error) {
	order := &model.BotOrder{}
	err := database.GetDB().Where("id = ? AND owner_id = ?", id, ownerId).First(order).Error
	return order, err
}

// CreateOrder is called from the bot's buyer flow once a plan and payment
// method have both been chosen.
func (s *BotService) CreateOrder(ownerId, planId int, buyerTgId int64, buyerUsername string, paymentMethodId int) (*model.BotOrder, error) {
	order := &model.BotOrder{
		OwnerId:         ownerId,
		PlanId:          planId,
		BuyerTgId:       buyerTgId,
		BuyerUsername:   buyerUsername,
		PaymentMethodId: paymentMethodId,
		Status:          model.BotOrderPendingPayment,
	}
	if err := database.GetDB().Create(order).Error; err != nil {
		return nil, err
	}
	return order, nil
}

// SubmitProof records the buyer's payment proof (text and/or a Telegram
// photo file_id) and moves the order into the review queue.
func (s *BotService) SubmitProof(orderId int, proofText, proofTgFileId string) error {
	result := database.GetDB().Model(&model.BotOrder{}).
		Where("id = ? AND status = ?", orderId, model.BotOrderPendingPayment).
		Updates(map[string]any{
			"status":           model.BotOrderPendingReview,
			"proof_text":       proofText,
			"proof_tg_file_id": proofTgFileId,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("order is not awaiting payment")
	}
	return nil
}

// ApproveOrder validates the plan is still within the owner's allowed
// inbounds and usage quota, provisions the client, and marks the order
// approved. Returns the generated client email so the caller can message it
// to the buyer.
func (s *BotService) ApproveOrder(orderId, reviewerId int) (*model.BotOrder, string, error) {
	db := database.GetDB()
	order := &model.BotOrder{}
	if err := db.First(order, orderId).Error; err != nil {
		return nil, "", err
	}
	if order.Status != model.BotOrderPendingReview {
		return nil, "", errors.New("order is not pending review")
	}
	plan := &model.BotPlan{}
	if err := db.First(plan, order.PlanId).Error; err != nil {
		return nil, "", err
	}
	owner := &model.User{}
	if err := db.First(owner, order.OwnerId).Error; err != nil {
		return nil, "", err
	}
	if owner.Role == "reseller" && owner.AllowedInboundsMode == "select" {
		allowed := make(map[int]bool, len(owner.AllowedInboundIds))
		for _, id := range owner.AllowedInboundIds {
			allowed[id] = true
		}
		for _, id := range plan.InboundIds {
			if !allowed[id] {
				return nil, "", errors.New("plan references an inbound no longer allowed for this reseller")
			}
		}
	}
	if owner.UsageLimit > 0 && owner.UsageUp+owner.UsageDown >= owner.UsageLimit {
		return nil, "", errors.New("reseller is over their usage quota, cannot approve new orders")
	}

	email := "bot-" + random.NumLower(10)
	client := model.Client{
		Email:      email,
		Enable:     true,
		TotalGB:    plan.TrafficGB * bytesPerGB,
		ExpiryTime: -(int64(plan.DurationDays) * msPerDay),
		OwnerId:    order.OwnerId,
	}
	inboundSvc := &service.InboundService{}
	clientSvc := &service.ClientService{}
	if _, err := clientSvc.Create(inboundSvc, &service.ClientCreatePayload{
		Client:     client,
		InboundIds: plan.InboundIds,
	}); err != nil {
		return nil, "", err
	}

	now := time.Now().UnixMilli()
	if err := db.Model(order).Updates(map[string]any{
		"status":       model.BotOrderApproved,
		"client_email": email,
		"reviewed_at":  now,
		"reviewed_by":  reviewerId,
	}).Error; err != nil {
		return nil, "", err
	}
	order.Status = model.BotOrderApproved
	order.ClientEmail = email
	return order, email, nil
}

func (s *BotService) RejectOrder(orderId, reviewerId int, reason string) (*model.BotOrder, error) {
	db := database.GetDB()
	order := &model.BotOrder{}
	if err := db.First(order, orderId).Error; err != nil {
		return nil, err
	}
	if order.Status != model.BotOrderPendingReview {
		return nil, errors.New("order is not pending review")
	}
	now := time.Now().UnixMilli()
	if err := db.Model(order).Updates(map[string]any{
		"status":        model.BotOrderRejected,
		"reject_reason": reason,
		"reviewed_at":   now,
		"reviewed_by":   reviewerId,
	}).Error; err != nil {
		return nil, err
	}
	order.Status = model.BotOrderRejected
	order.RejectReason = reason
	return order, nil
}

// ExpireStalePendingOrders marks pending_payment orders older than the TTL
// as expired — the buyer picked a plan/method but never sent proof.
func (s *BotService) ExpireStalePendingOrders() (int64, error) {
	cutoff := time.Now().Add(-staleOrderTTL).UnixMilli()
	result := database.GetDB().Model(&model.BotOrder{}).
		Where("status = ? AND created_at < ?", model.BotOrderPendingPayment, cutoff).
		Update("status", model.BotOrderExpired)
	return result.RowsAffected, result.Error
}
