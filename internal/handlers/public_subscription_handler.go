package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/mp"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucSubscription "github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

type PublicSubscriptionHandler struct {
	db          *gorm.DB
	listPlansUC *ucSubscription.ListPlans
	purchaseUC  *ucSubscription.PurchaseSubscription
}

func NewPublicSubscriptionHandler(
	db *gorm.DB,
	listPlansUC *ucSubscription.ListPlans,
	purchaseUC *ucSubscription.PurchaseSubscription,
) *PublicSubscriptionHandler {
	return &PublicSubscriptionHandler{
		db:          db,
		listPlansUC: listPlansUC,
		purchaseUC:  purchaseUC,
	}
}

// ──────────────────────────────────────────────────────────────────
// GET /api/public/:slug/plans
// ──────────────────────────────────────────────────────────────────

func (h *PublicSubscriptionHandler) ListPlans(c *gin.Context) {
	shop, ok := h.resolveShop(c)
	if !ok {
		return
	}

	plans, err := h.listPlansUC.Execute(c.Request.Context(), shop.ID)
	if err != nil {
		httperr.Internal(c, "failed_to_list_plans", "Erro ao listar planos.")
		return
	}

	type planDTO struct {
		ID                uint  `json:"id"`
		Name              string `json:"name"`
		MonthlyPriceCents int64  `json:"monthly_price_cents"`
		DurationDays      int    `json:"duration_days"`
		CutsIncluded      int    `json:"cuts_included"`
		DiscountPercent   int    `json:"discount_percent"`
		ServiceIDs        []uint `json:"service_ids"`
		CategoryIDs       []uint `json:"category_ids"`
	}

	dtos := make([]planDTO, 0, len(plans))
	for _, p := range plans {
		if !p.Active {
			continue
		}
		dtos = append(dtos, planDTO{
			ID:                p.ID,
			Name:              p.Name,
			MonthlyPriceCents: p.MonthlyPriceCents,
			DurationDays:      p.DurationDays,
			CutsIncluded:      p.CutsIncluded,
			DiscountPercent:   p.DiscountPercent,
			ServiceIDs:        p.ServiceIDs,
			CategoryIDs:       p.CategoryIDs,
		})
	}

	setCacheControl(c, 120)
	c.JSON(http.StatusOK, gin.H{"plans": dtos})
}

// ──────────────────────────────────────────────────────────────────
// POST /api/public/:slug/subscriptions/purchase
// ──────────────────────────────────────────────────────────────────

type purchaseSubscriptionRequest struct {
	PlanID          uint   `json:"plan_id"          binding:"required"`
	ClientName      string `json:"client_name"      binding:"required"`
	ClientPhone     string `json:"client_phone"     binding:"required"`
	PayerEmail      string `json:"payer_email"      binding:"required,email"`
	PayerCPF        string `json:"payer_cpf"`
	PaymentMethodID string `json:"payment_method_id" binding:"required"`
	Token           string `json:"token"`
	Installments    int    `json:"installments"`
}

type purchaseSubscriptionResponse struct {
	SubscriptionID uint   `json:"subscription_id"`
	PaymentID      uint   `json:"payment_id"`
	MPPaymentID    int64  `json:"mp_payment_id"`
	Status         string `json:"status"`
	// PIX
	QRCode       string `json:"qr_code,omitempty"`
	QRCodeBase64 string `json:"qr_code_base64,omitempty"`
	TicketURL    string `json:"ticket_url,omitempty"`
}

func (h *PublicSubscriptionHandler) Purchase(c *gin.Context) {
	shop, ok := h.resolveShop(c)
	if !ok {
		return
	}

	// Garante que MP está configurado
	var paymentCfg models.BarbershopPaymentConfig
	hasCfg := h.db.WithContext(c.Request.Context()).
		Where("barbershop_id = ?", shop.ID).
		First(&paymentCfg).Error == nil

	if !hasCfg || paymentCfg.MPAccessToken == "" || paymentCfg.MPPublicKey == "" {
		httperr.BadRequest(c, "payment_not_configured", "Esta barbearia ainda não configurou o pagamento online.")
		return
	}

	var req purchaseSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "Dados inválidos.")
		return
	}

	if req.Installments <= 0 {
		req.Installments = 1
	}

	input := ucSubscription.PurchaseSubscriptionInput{
		BarbershopID:    shop.ID,
		PlanID:          req.PlanID,
		ClientName:      req.ClientName,
		ClientPhone:     req.ClientPhone,
		PayerEmail:      req.PayerEmail,
		PayerCPF:        req.PayerCPF,
		PaymentMethodID: req.PaymentMethodID,
		Token:           req.Token,
		Installments:    req.Installments,
	}

	// Usa o gateway da própria barbearia quando disponível
	var result *ucSubscription.PurchaseSubscriptionResult
	var err error
	if gw, gwErr := mp.New(paymentCfg.MPAccessToken); gwErr == nil {
		result, err = h.purchaseUC.Execute(c.Request.Context(), input, gw)
	} else {
		result, err = h.purchaseUC.Execute(c.Request.Context(), input)
	}
	if err != nil {
		switch {
		case httperr.IsBusiness(err, "plan_not_found"):
			httperr.BadRequest(c, "plan_not_found", "Plano não encontrado.")
		case httperr.IsBusiness(err, "client_already_has_active_subscription"):
			httperr.Write(c, http.StatusConflict, "already_subscribed", "Este cliente já possui uma assinatura ativa.")
		case httperr.IsBusiness(err, "payment_rejected"):
			httperr.BadRequest(c, "payment_rejected", "Pagamento recusado. Verifique os dados do cartão.")
		default:
			log.Printf("[purchase] unexpected error: %v", err)
			httperr.Internal(c, "purchase_failed", "Erro ao processar assinatura.")
		}
		return
	}

	c.JSON(http.StatusCreated, purchaseSubscriptionResponse{
		SubscriptionID: result.SubscriptionID,
		PaymentID:      result.PaymentID,
		MPPaymentID:    result.MPPaymentID,
		Status:         result.Status,
		QRCode:         result.QRCode,
		QRCodeBase64:   result.QRCodeBase64,
		TicketURL:      result.TicketURL,
	})
}

// ──────────────────────────────────────────────────────────────────
// GET /api/public/:slug/subscriptions/:id/payment/status
// Polling para PIX — retorna status do pagamento/subscription
// ──────────────────────────────────────────────────────────────────

func (h *PublicSubscriptionHandler) PaymentStatus(c *gin.Context) {
	shop, ok := h.resolveShop(c)
	if !ok {
		return
	}

	subID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || subID64 == 0 {
		httperr.BadRequest(c, "invalid_subscription_id", "ID inválido.")
		return
	}

	var sub models.Subscription
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND barbershop_id = ?", uint(subID64), shop.ID).
		First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.NotFound(c, "subscription_not_found", "Assinatura não encontrada.")
			return
		}
		httperr.Internal(c, "failed_to_load_subscription", "Erro ao carregar assinatura.")
		return
	}

	// Se ainda pendente, tenta verificar o status no MP e ativar inline.
	if sub.Status == "pending_payment" {
		h.tryActivateFromMP(c.Request.Context(), &sub, shop)
	}

	setNoStore(c)
	c.JSON(http.StatusOK, gin.H{
		"subscription_id": sub.ID,
		"status":          sub.Status,
	})
}

// tryActivateFromMP consulta o MP pelo mp_payment_id armazenado no pagamento.
// Se aprovado, ativa o pagamento e a assinatura dentro de uma transação.
// Atualiza sub.Status in-place para que o mesmo request já retorne "active".
func (h *PublicSubscriptionHandler) tryActivateFromMP(ctx context.Context, sub *models.Subscription, shop *models.Barbershop) {
	// 1. Encontra o pagamento vinculado com mp_payment_id preenchido
	var pmt models.Payment
	if err := h.db.WithContext(ctx).
		Where("subscription_id = ? AND mp_payment_id IS NOT NULL", sub.ID).
		First(&pmt).Error; err != nil {
		return // pagamento ainda não tem mp_payment_id — aguarda próximo ciclo
	}

	// 2. Busca config MP da barbearia
	var paymentCfg models.BarbershopPaymentConfig
	if err := h.db.WithContext(ctx).
		Where("barbershop_id = ?", shop.ID).
		First(&paymentCfg).Error; err != nil || paymentCfg.MPAccessToken == "" {
		return
	}

	// 3. Consulta o MP
	gw, err := mp.New(paymentCfg.MPAccessToken)
	if err != nil {
		return
	}
	mpStatus, err := gw.GetPaymentStatus(*pmt.MPPaymentID)
	if err != nil || mpStatus != "approved" {
		return
	}

	// 4. Ativa dentro de transação
	now := time.Now().UTC()
	var plan models.Plan
	if err := h.db.WithContext(ctx).Where("id = ?", sub.PlanID).First(&plan).Error; err != nil {
		return
	}
	periodStart := now
	periodEnd := periodStart.AddDate(0, 0, plan.DurationDays)

	txErr := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Subscription{}).
			Where("id = ? AND status = ?", sub.ID, "pending_payment").
			Updates(map[string]any{
				"status":                  "active",
				"current_period_start":    periodStart,
				"current_period_end":      periodEnd,
				"cuts_used_in_period":     0,
				"cuts_reserved_in_period": 0,
			}).Error; err != nil {
			return err
		}
		paidAt := now
		return tx.Model(&models.Payment{}).
			Where("id = ?", pmt.ID).
			Updates(map[string]any{"status": "paid", "paid_at": paidAt}).Error
	})
	if txErr != nil {
		log.Printf("[paymentstatus] failed to activate subscription %d: %v", sub.ID, txErr)
		return
	}

	sub.Status = "active"
}

// ──────────────────────────────────────────────────────────────────
// GET /api/public/:slug/subscribers/lookup?phone=:phone
// ──────────────────────────────────────────────────────────────────

func (h *PublicSubscriptionHandler) LookupSubscriber(c *gin.Context) {
	shop, ok := h.resolveShop(c)
	if !ok {
		return
	}

	phone := strings.Join(strings.Fields(c.Query("phone")), "")
	if phone == "" {
		httperr.BadRequest(c, "phone_required", "Telefone obrigatório.")
		return
	}

	// Busca cliente pelo telefone
	var client models.Client
	err := h.db.WithContext(c.Request.Context()).
		Where("barbershop_id = ? AND phone = ?", shop.ID, phone).
		First(&client).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		setNoStore(c)
		c.JSON(http.StatusOK, gin.H{"found": false})
		return
	}
	if err != nil {
		httperr.Internal(c, "lookup_failed", "Erro ao consultar.")
		return
	}

	// Busca assinatura ativa (dentro do período)
	now := time.Now().UTC()
	var sub models.Subscription
	err = h.db.WithContext(c.Request.Context()).
		Where(
			"barbershop_id = ? AND client_id = ? AND status = ? AND current_period_start <= ? AND current_period_end > ?",
			shop.ID, client.ID, "active", now, now,
		).
		Order("current_period_end DESC").
		First(&sub).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Verifica se existe assinatura expirada/cancelada
		var expiredSub models.Subscription
		expiredErr := h.db.WithContext(c.Request.Context()).
			Where("barbershop_id = ? AND client_id = ? AND status IN ?", shop.ID, client.ID, []string{"expired", "cancelled"}).
			Order("current_period_end DESC").
			First(&expiredSub).Error

		if expiredErr == nil {
			var expiredPlan models.Plan
			_ = h.db.WithContext(c.Request.Context()).Where("id = ?", expiredSub.PlanID).First(&expiredPlan).Error
			setNoStore(c)
			c.JSON(http.StatusOK, gin.H{
				"found":               true,
				"subscriber_name":     client.Name,
				"plan_name":           expiredPlan.Name,
				"subscription_status": "expired",
				"cuts_used":           expiredSub.CutsUsedInPeriod,
				"cuts_included":       expiredPlan.CutsIncluded,
				"period_end_date":     expiredSub.CurrentPeriodEnd.Format("02/01/2006"),
				"covered_service_ids": []uint{},
			})
			return
		}

		setNoStore(c)
		c.JSON(http.StatusOK, gin.H{"found": false})
		return
	}
	if err != nil {
		httperr.Internal(c, "lookup_failed", "Erro ao consultar assinatura.")
		return
	}

	// Busca plano
	var plan models.Plan
	if err := h.db.WithContext(c.Request.Context()).Where("id = ?", sub.PlanID).First(&plan).Error; err != nil {
		setNoStore(c)
		c.JSON(http.StatusOK, gin.H{"found": false})
		return
	}

	// Verifica se esgotado
	totalCommitted := sub.CutsUsedInPeriod + sub.CutsReservedInPeriod
	if plan.CutsIncluded > 0 && totalCommitted >= plan.CutsIncluded {
		setNoStore(c)
		c.JSON(http.StatusOK, gin.H{
			"found":               true,
			"subscriber_name":     client.Name,
			"plan_name":           plan.Name,
			"subscription_status": "exhausted",
			"cuts_used":           sub.CutsUsedInPeriod,
			"cuts_included":       plan.CutsIncluded,
			"period_end_date":     sub.CurrentPeriodEnd.Format("02/01/2006"),
			"covered_service_ids": []uint{},
		})
		return
	}

	// Serviços cobertos pelo plano
	var serviceIDs []uint
	h.db.WithContext(c.Request.Context()).Raw(`
		SELECT DISTINCT bs.id
		FROM barbershop_services bs
		WHERE bs.id IN (SELECT service_id FROM plan_services WHERE plan_id = ?)
		   OR (bs.category_id IS NOT NULL AND bs.category_id IN (
		       SELECT category_id FROM plan_categories WHERE plan_id = ?
		   ))
	`, sub.PlanID, sub.PlanID).Scan(&serviceIDs)

	if serviceIDs == nil {
		serviceIDs = []uint{}
	}

	setNoStore(c)
	c.JSON(http.StatusOK, gin.H{
		"found":               true,
		"subscriber_name":     client.Name,
		"plan_name":           plan.Name,
		"subscription_status": "active",
		"cuts_used":           sub.CutsUsedInPeriod,
		"cuts_included":       plan.CutsIncluded,
		"period_end_date":     sub.CurrentPeriodEnd.Format("02/01/2006"),
		"covered_service_ids": serviceIDs,
	})
}

// ──────────────────────────────────────────────────────────────────
// helpers
// ──────────────────────────────────────────────────────────────────

func (h *PublicSubscriptionHandler) resolveShop(c *gin.Context) (*models.Barbershop, bool) {
	slug := c.Param("slug")
	var shop models.Barbershop
	if err := h.db.WithContext(c.Request.Context()).
		Where("slug = ?", slug).
		First(&shop).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
			return nil, false
		}
		httperr.Internal(c, "failed_to_load_barbershop", "Erro ao carregar barbearia.")
		return nil, false
	}
	return &shop, true
}
