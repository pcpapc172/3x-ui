package panel

import (
	"errors"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func (s *BotService) ListOrders(ownerId int, status string) ([]model.BotOrder, error) {
	q := database.GetDB().Where("owner_id = ?", ownerId)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var orders []model.BotOrder
	err := q.Order("created_at DESC").Find(&orders).Error
	return orders, err
}

// ListOrdersForBuyer is the buyer-facing "my invoices" list — ListOrders
// above returns every buyer's orders and is only for the owner's review UI.
func (s *BotService) ListOrdersForBuyer(ownerId int, buyerTgId int64) ([]model.BotOrder, error) {
	var orders []model.BotOrder
	err := database.GetDB().Where("owner_id = ? AND buyer_tg_id = ?", ownerId, buyerTgId).
		Order("created_at DESC").Find(&orders).Error
	return orders, err
}

func (s *BotService) GetOrder(ownerId, id int) (*model.BotOrder, error) {
	order := &model.BotOrder{}
	err := database.GetDB().Where("id = ? AND owner_id = ?", id, ownerId).First(order).Error
	return order, err
}

func (s *BotService) newOrder(ownerId int, kind string, planId, subscriptionId int, buyerTgId int64, buyerUsername string, paymentMethodId int, amountToman int64) *model.BotOrder {
	return &model.BotOrder{
		OwnerId:          ownerId,
		Kind:             kind,
		PlanId:           planId,
		SubscriptionId:   subscriptionId,
		BuyerTgId:        buyerTgId,
		BuyerUsername:    buyerUsername,
		PaymentMethodId:  paymentMethodId,
		Status:           model.BotOrderPendingPayment,
		FinalAmountToman: amountToman,
		PublicToken:      random.Seq(12),
	}
}

// applyDiscount looks up code (scoped to the order's owner/plan), and on
// success sets the order's discount fields and reduces FinalAmountToman.
func (s *BotService) applyDiscount(order *model.BotOrder, code string, priceToman int64) error {
	if code == "" {
		return nil
	}
	dc, off, err := s.ValidateDiscountCode(order.OwnerId, order.PlanId, code, priceToman)
	if err != nil {
		return err
	}
	order.DiscountCodeId = dc.Id
	order.DiscountAmountToman = off
	order.FinalAmountToman = priceToman - off
	return nil
}

// CreatePurchaseOrder is called from the bot's buyer flow once a plan and
// payment method have both been chosen.
func (s *BotService) CreatePurchaseOrder(ownerId, planId int, buyerTgId int64, buyerUsername string, paymentMethodId int, discountCode string) (*model.BotOrder, error) {
	plan := &model.BotPlan{}
	if err := database.GetDB().Where("id = ? AND owner_id = ?", planId, ownerId).First(plan).Error; err != nil {
		return nil, err
	}
	order := s.newOrder(ownerId, model.BotOrderKindPurchase, planId, 0, buyerTgId, buyerUsername, paymentMethodId, plan.PriceToman)
	if err := s.applyDiscount(order, discountCode, plan.PriceToman); err != nil {
		return nil, err
	}
	if err := database.GetDB().Create(order).Error; err != nil {
		return nil, err
	}
	return order, nil
}

// CreateRenewalOrder extends an existing subscription's client rather than
// provisioning a new one — see settleOrder's BotOrderKindRenewal branch.
func (s *BotService) CreateRenewalOrder(ownerId, subscriptionId int, buyerTgId int64, buyerUsername string, paymentMethodId int, discountCode string) (*model.BotOrder, error) {
	sub := &model.BotSubscription{}
	if err := database.GetDB().Where("id = ? AND owner_id = ?", subscriptionId, ownerId).First(sub).Error; err != nil {
		return nil, err
	}
	plan := &model.BotPlan{}
	if err := database.GetDB().First(plan, sub.PlanId).Error; err != nil {
		return nil, err
	}
	order := s.newOrder(ownerId, model.BotOrderKindRenewal, sub.PlanId, sub.Id, buyerTgId, buyerUsername, paymentMethodId, plan.PriceToman)
	if err := s.applyDiscount(order, discountCode, plan.PriceToman); err != nil {
		return nil, err
	}
	if err := database.GetDB().Create(order).Error; err != nil {
		return nil, err
	}
	return order, nil
}

// CreateTopupOrder starts a wallet top-up invoice; approving it (via owner
// review or, once that becomes possible, another rail) credits the wallet
// instead of provisioning a client — see settleOrder.
func (s *BotService) CreateTopupOrder(ownerId int, buyerTgId int64, buyerUsername string, amountToman int64, paymentMethodId int) (*model.BotOrder, error) {
	if amountToman <= 0 {
		return nil, errors.New("top-up amount must be positive")
	}
	order := s.newOrder(ownerId, model.BotOrderKindTopup, 0, 0, buyerTgId, buyerUsername, paymentMethodId, amountToman)
	if err := database.GetDB().Create(order).Error; err != nil {
		return nil, err
	}
	return order, nil
}

// ApplyDiscountToOrder validates and attaches a discount code to an
// already-created, still-unpaid order (the buyer types the code as a
// separate step after the invoice is created, matching the invoice card's
// "apply discount" button).
func (s *BotService) ApplyDiscountToOrder(orderId int, code string) (*model.BotOrder, error) {
	db := database.GetDB()
	order := &model.BotOrder{}
	if err := db.First(order, orderId).Error; err != nil {
		return nil, err
	}
	if order.Status != model.BotOrderPendingPayment {
		return nil, errors.New("order is not awaiting payment")
	}
	basePrice := order.FinalAmountToman + order.DiscountAmountToman
	dc, off, err := s.ValidateDiscountCode(order.OwnerId, order.PlanId, code, basePrice)
	if err != nil {
		return nil, err
	}
	final := basePrice - off
	if err := db.Model(order).Updates(map[string]any{
		"discount_code_id":      dc.Id,
		"discount_amount_toman": off,
		"final_amount_toman":    final,
	}).Error; err != nil {
		return nil, err
	}
	order.DiscountCodeId = dc.Id
	order.DiscountAmountToman = off
	order.FinalAmountToman = final
	return order, nil
}

// SetOrderPaymentMethod records the buyer's chosen card/crypto method on an
// already-created order (chosen after the invoice, not at creation time).
func (s *BotService) SetOrderPaymentMethod(orderId, methodId int) error {
	result := database.GetDB().Model(&model.BotOrder{}).
		Where("id = ? AND status = ?", orderId, model.BotOrderPendingPayment).
		Update("payment_method_id", methodId)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("order is not awaiting payment")
	}
	return nil
}

// SubmitProof records the buyer's payment proof (text, an optional Telegram
// photo file_id, and an optional bank transaction reference) and moves the
// order into the review queue.
func (s *BotService) SubmitProof(orderId int, proofText, proofTgFileId, transactionRef string) error {
	result := database.GetDB().Model(&model.BotOrder{}).
		Where("id = ? AND status = ?", orderId, model.BotOrderPendingPayment).
		Updates(map[string]any{
			"status":           model.BotOrderPendingReview,
			"proof_text":       proofText,
			"proof_tg_file_id": proofTgFileId,
			"transaction_ref":  transactionRef,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("order is not awaiting payment")
	}
	return nil
}

// CancelOrder lets a buyer withdraw their own invoice before it's approved.
func (s *BotService) CancelOrder(ownerId, orderId int) error {
	result := database.GetDB().Model(&model.BotOrder{}).
		Where("id = ? AND owner_id = ? AND status IN ?", orderId, ownerId,
			[]string{model.BotOrderPendingPayment, model.BotOrderPendingReview}).
		Update("status", model.BotOrderCanceled)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("order cannot be canceled in its current state")
	}
	return nil
}

// settleOrder performs the actual effect of an approval: provisions a new
// client (purchase), extends an existing one without touching its used
// traffic (renewal — see internal/web/service ClientService.BulkAdjust), or
// credits the wallet (topup). Shared by the owner-reviewed ApproveOrder path
// and the instant PayOrderFromWallet path.
func (s *BotService) settleOrder(order *model.BotOrder) (string, error) {
	switch order.Kind {
	case model.BotOrderKindTopup:
		customer, err := s.GetOrCreateCustomer(order.OwnerId, order.BuyerTgId, order.BuyerUsername)
		if err != nil {
			return "", err
		}
		if err := s.CreditWallet(order.OwnerId, customer.Id, model.BotWalletTxTopup,
			order.FinalAmountToman, order.Id, "wallet top-up"); err != nil {
			return "", err
		}
		return "", nil

	case model.BotOrderKindRenewal:
		sub := &model.BotSubscription{}
		if err := database.GetDB().First(sub, order.SubscriptionId).Error; err != nil {
			return "", err
		}
		plan := &model.BotPlan{}
		if err := database.GetDB().First(plan, order.PlanId).Error; err != nil {
			return "", err
		}
		inboundSvc := &service.InboundService{}
		clientSvc := &service.ClientService{}
		result, _, err := clientSvc.BulkAdjust(inboundSvc, []string{sub.ClientEmail}, plan.DurationDays, plan.TrafficGB*bytesPerGB, "")
		if err != nil {
			return "", err
		}
		for _, skip := range result.Skipped {
			if skip.Email == sub.ClientEmail && !strings.Contains(skip.Reason, "unlimited") {
				return "", errors.New("renewal failed: " + skip.Reason)
			}
		}
		return sub.ClientEmail, nil

	default: // purchase
		plan := &model.BotPlan{}
		if err := database.GetDB().First(plan, order.PlanId).Error; err != nil {
			return "", err
		}
		owner := &model.User{}
		if err := database.GetDB().First(owner, order.OwnerId).Error; err != nil {
			return "", err
		}
		if owner.Role == "reseller" && owner.AllowedInboundsMode == "select" {
			allowed := make(map[int]bool, len(owner.AllowedInboundIds))
			for _, id := range owner.AllowedInboundIds {
				allowed[id] = true
			}
			for _, id := range plan.InboundIds {
				if !allowed[id] {
					return "", errors.New("plan references an inbound no longer allowed for this reseller")
				}
			}
		}
		if owner.UsageLimit > 0 && owner.UsageUp+owner.UsageDown >= owner.UsageLimit {
			return "", errors.New("reseller is over their usage quota, cannot approve new orders")
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
			return "", err
		}

		customer, err := s.GetOrCreateCustomer(order.OwnerId, order.BuyerTgId, order.BuyerUsername)
		if err != nil {
			return "", err
		}
		if err := database.GetDB().Create(&model.BotSubscription{
			OwnerId:     order.OwnerId,
			CustomerId:  customer.Id,
			ClientEmail: email,
			PlanId:      plan.Id,
		}).Error; err != nil {
			return "", err
		}
		return email, nil
	}
}

// afterSettled increments the discount code's use count and applies any
// referral bonus — the bookkeeping common to both approval paths, run only
// once an order has actually settled.
func (s *BotService) afterSettled(order *model.BotOrder) {
	if order.DiscountCodeId != 0 {
		_ = s.IncrementDiscountCodeUse(order.DiscountCodeId)
	}
	if order.Kind == model.BotOrderKindPurchase {
		_ = s.ApplyReferralBonus(order)
	}
}

// PayOrderFromWallet debits the buyer's wallet and settles the order
// immediately, with no owner review — the funds were already verified when
// the wallet was topped up. On settlement failure the debit is refunded.
func (s *BotService) PayOrderFromWallet(orderId int) (*model.BotOrder, string, error) {
	db := database.GetDB()
	order := &model.BotOrder{}
	if err := db.First(order, orderId).Error; err != nil {
		return nil, "", err
	}
	if order.Status != model.BotOrderPendingPayment {
		return nil, "", errors.New("order is not awaiting payment")
	}
	customer, err := s.GetOrCreateCustomer(order.OwnerId, order.BuyerTgId, order.BuyerUsername)
	if err != nil {
		return nil, "", err
	}
	if err := s.DebitWallet(order.OwnerId, customer.Id, model.BotWalletTxPurchase,
		order.FinalAmountToman, order.Id, "order "+order.PublicToken); err != nil {
		return nil, "", err
	}
	clientEmail, err := s.settleOrder(order)
	if err != nil {
		_ = s.CreditWallet(order.OwnerId, customer.Id, model.BotWalletTxRefund,
			order.FinalAmountToman, order.Id, "refund: settlement failed")
		return nil, "", err
	}
	now := time.Now().UnixMilli()
	if err := db.Model(order).Updates(map[string]any{
		"status":          model.BotOrderApproved,
		"client_email":    clientEmail,
		"pay_from_wallet": true,
		"paid_at":         now,
		"reviewed_at":     now,
	}).Error; err != nil {
		return nil, "", err
	}
	order.Status = model.BotOrderApproved
	order.ClientEmail = clientEmail
	order.PayFromWallet = true
	s.afterSettled(order)
	return order, clientEmail, nil
}

// ApproveOrder validates a pending_review order (proof already submitted for
// card/crypto payment), settles it, and marks it approved. Returns the
// affected client's email so the caller can message it to the buyer.
func (s *BotService) ApproveOrder(orderId, reviewerId int) (*model.BotOrder, string, error) {
	db := database.GetDB()
	order := &model.BotOrder{}
	if err := db.First(order, orderId).Error; err != nil {
		return nil, "", err
	}
	if order.Status != model.BotOrderPendingReview {
		return nil, "", errors.New("order is not pending review")
	}
	clientEmail, err := s.settleOrder(order)
	if err != nil {
		return nil, "", err
	}
	now := time.Now().UnixMilli()
	if err := db.Model(order).Updates(map[string]any{
		"status":       model.BotOrderApproved,
		"client_email": clientEmail,
		"paid_at":      now,
		"reviewed_at":  now,
		"reviewed_by":  reviewerId,
	}).Error; err != nil {
		return nil, "", err
	}
	order.Status = model.BotOrderApproved
	order.ClientEmail = clientEmail
	s.afterSettled(order)
	return order, clientEmail, nil
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

// ClaimTrial provisions the owner's trial plan (BotPlan.IsTrial) for a
// buyer's first-ever trial request, at zero price and with no payment step —
// settled the same way an approved order is, and gated by BotCustomer.TrialUsed
// so a buyer can only ever claim one.
func (s *BotService) ClaimTrial(ownerId int, buyerTgId int64, buyerUsername string) (string, error) {
	db := database.GetDB()
	plan := &model.BotPlan{}
	if err := db.Where("owner_id = ? AND is_trial = ? AND enabled = ?", ownerId, true, true).First(plan).Error; err != nil {
		return "", errors.New("no trial plan available")
	}
	customer, err := s.GetOrCreateCustomer(ownerId, buyerTgId, buyerUsername)
	if err != nil {
		return "", err
	}
	if customer.TrialUsed {
		return "", errors.New("trial already used")
	}
	order := s.newOrder(ownerId, model.BotOrderKindPurchase, plan.Id, 0, buyerTgId, buyerUsername, 0, 0)
	if err := db.Create(order).Error; err != nil {
		return "", err
	}
	email, err := s.settleOrder(order)
	if err != nil {
		return "", err
	}
	now := time.Now().UnixMilli()
	if err := db.Model(order).Updates(map[string]any{
		"status":       model.BotOrderApproved,
		"client_email": email,
		"paid_at":      now,
		"reviewed_at":  now,
	}).Error; err != nil {
		return "", err
	}
	if err := db.Model(&model.BotCustomer{}).Where("id = ?", customer.Id).Update("trial_used", true).Error; err != nil {
		return "", err
	}
	return email, nil
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
