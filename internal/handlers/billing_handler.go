package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	mpSDKConfig "github.com/mercadopago/sdk-go/pkg/config"
	mpPayment "github.com/mercadopago/sdk-go/pkg/payment"
	mpPreference "github.com/mercadopago/sdk-go/pkg/preference"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type BillingHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewBillingHandler(db *gorm.DB, cfg *config.Config) *BillingHandler {
	return &BillingHandler{db: db, cfg: cfg}
}

// GET /api/me/billing/status
// Returns current subscription status. Bypasses subscription check in AuthMiddleware.
func (h *BillingHandler) Status(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var shop models.Barbershop
	if err := h.db.
		Select("id, status, trial_ends_at, subscription_expires_at").
		First(&shop, barbershopID).Error; err != nil {
		httperr.NotFound(c, "not_found", "Barbearia não encontrada.")
		return
	}

	now := time.Now()
	var daysRemaining *int
	var expiresAt *time.Time

	switch shop.Status {
	case "trial":
		if shop.TrialEndsAt != nil {
			d := int(shop.TrialEndsAt.Sub(now).Hours() / 24)
			if d < 0 {
				d = 0
			}
			daysRemaining = &d
			expiresAt = shop.TrialEndsAt
		}
	case "active":
		if shop.SubscriptionExpiresAt != nil {
			d := int(shop.SubscriptionExpiresAt.Sub(now).Hours() / 24)
			if d < 0 {
				d = 0
			}
			daysRemaining = &d
			expiresAt = shop.SubscriptionExpiresAt
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":               shop.Status,
		"days_remaining":       daysRemaining,
		"expires_at":           expiresAt,
		"monthly_price_cents":  h.cfg.PlatformMonthlyPriceCents,
		"mp_public_key":        h.cfg.PlatformMPPublicKey,
	})
}

// POST /api/me/billing/checkout
// Creates a Mercado Pago Checkout Pro preference for the platform subscription.
// In mock mode, activates the subscription immediately and returns the success URL.
func (h *BillingHandler) Checkout(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var shop models.Barbershop
	if err := h.db.First(&shop, barbershopID).Error; err != nil {
		httperr.NotFound(c, "not_found", "Barbearia não encontrada.")
		return
	}

	successURL := fmt.Sprintf("%s/app/billing/sucesso", h.cfg.AppURL)
	pendingURL := fmt.Sprintf("%s/app/billing/pendente", h.cfg.AppURL)
	failureURL := fmt.Sprintf("%s/app/billing", h.cfg.AppURL)

	// Mock mode: activate immediately and redirect to success.
	if h.cfg.MPProvider != "mp" {
		if err := h.activateBarbershop(barbershopID); err != nil {
			httperr.Internal(c, "activation_error", "Erro ao ativar conta.")
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"init_point":    successURL,
			"sandbox_point": successURL,
			"preference_id": "mock",
		})
		return
	}

	mpCfg, err := mpSDKConfig.New(h.cfg.MPAccessToken)
	if err != nil {
		httperr.Internal(c, "mp_config_error", "Erro ao configurar gateway de pagamento.")
		return
	}

	prefClient := mpPreference.NewClient(mpCfg)
	amount := float64(h.cfg.PlatformMonthlyPriceCents) / 100
	externalRef := fmt.Sprintf("billing:%d", barbershopID)
	notificationURL := fmt.Sprintf("%s/api/billing/webhook", h.cfg.BackendURL)

	resp, err := prefClient.Create(context.Background(), mpPreference.Request{
		Items: []mpPreference.ItemRequest{
			{
				Title:      fmt.Sprintf("Mensalidade Corteon — %s", shop.Name),
				Quantity:   1,
				UnitPrice:  amount,
				CurrencyID: "BRL",
			},
		},
		BackURLs: &mpPreference.BackURLsRequest{
			Success: successURL,
			Pending: pendingURL,
			Failure: failureURL,
		},
		AutoReturn:        "approved",
		ExternalReference: externalRef,
		NotificationURL:   notificationURL,
	})
	if err != nil {
		log.Printf("[BillingCheckout] mp preference error: %v", err)
		httperr.Internal(c, "mp_preference_error", "Erro ao criar link de pagamento.")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"init_point":    resp.InitPoint,
		"sandbox_point": resp.SandboxInitPoint,
		"preference_id": resp.ID,
	})
}

// POST /api/billing/webhook (public — called by Mercado Pago)
func (h *BillingHandler) Webhook(c *gin.Context) {
	topic := c.Query("topic")
	idStr := c.Query("id")

	// Newer MP notification format (JSON body).
	var body struct {
		Type string `json:"type"`
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = c.ShouldBindJSON(&body)

	if topic == "" && body.Type != "" {
		topic = body.Type
		idStr = body.Data.ID
	}

	if topic != "payment" || idStr == "" {
		c.Status(http.StatusOK)
		return
	}

	paymentID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.Status(http.StatusOK)
		return
	}

	if h.cfg.MPProvider != "mp" {
		c.Status(http.StatusOK)
		return
	}

	mpCfg, err := mpSDKConfig.New(h.cfg.MPAccessToken)
	if err != nil {
		log.Printf("[BillingWebhook] mp config error: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	paymentClient := mpPayment.NewClient(mpCfg)
	pay, err := paymentClient.Get(context.Background(), int(paymentID))
	if err != nil {
		log.Printf("[BillingWebhook] payment get error: %v", err)
		c.Status(http.StatusOK)
		return
	}

	if !strings.HasPrefix(pay.ExternalReference, "billing:") {
		c.Status(http.StatusOK)
		return
	}

	if pay.Status != "approved" {
		c.Status(http.StatusOK)
		return
	}

	parts := strings.SplitN(pay.ExternalReference, ":", 2)
	if len(parts) != 2 {
		c.Status(http.StatusOK)
		return
	}

	barbershopID, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		c.Status(http.StatusOK)
		return
	}

	if err := h.activateBarbershop(uint(barbershopID)); err != nil {
		log.Printf("[BillingWebhook] failed to activate barbershop %d: %v", barbershopID, err)
		c.Status(http.StatusInternalServerError)
		return
	}

	log.Printf("[BillingWebhook] activated barbershop %d", barbershopID)
	c.Status(http.StatusOK)
}

type billingPayRequest struct {
	PayerEmail      string `json:"payer_email"       binding:"required,email"`
	PayerCPF        string `json:"payer_cpf"`
	PaymentMethodID string `json:"payment_method_id" binding:"required"`
	Token           string `json:"token"`
	Installments    int    `json:"installments"`
}

// POST /api/me/billing/pay
// Creates a transparent payment (PIX or credit card) for the platform subscription.
// In mock mode, activates the subscription immediately and returns approved.
func (h *BillingHandler) Pay(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	// Mock mode: activate immediately.
	if h.cfg.MPProvider != "mp" {
		if err := h.activateBarbershop(barbershopID); err != nil {
			httperr.Internal(c, "activation_error", "Erro ao ativar conta.")
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "approved", "mp_payment_id": 0})
		return
	}

	var req billingPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_body", err.Error())
		return
	}

	mpCfg, err := mpSDKConfig.New(h.cfg.MPAccessToken)
	if err != nil {
		httperr.Internal(c, "mp_config_error", "Erro ao configurar gateway de pagamento.")
		return
	}

	paymentClient := mpPayment.NewClient(mpCfg)
	amount := float64(h.cfg.PlatformMonthlyPriceCents) / 100
	externalRef := fmt.Sprintf("billing:%d", barbershopID)
	notificationURL := fmt.Sprintf("%s/api/billing/webhook", h.cfg.BackendURL)

	installments := req.Installments
	if installments <= 0 {
		installments = 1
	}

	pay, err := paymentClient.Create(context.Background(), mpPayment.Request{
		TransactionAmount: amount,
		Description:       "Mensalidade Corteon",
		ExternalReference: externalRef,
		NotificationURL:   notificationURL,
		PaymentMethodID:   req.PaymentMethodID,
		Token:             req.Token,
		Installments:      installments,
		Payer: &mpPayment.PayerRequest{
			Email: req.PayerEmail,
			Identification: &mpPayment.IdentificationRequest{
				Type:   "CPF",
				Number: req.PayerCPF,
			},
		},
	})
	if err != nil {
		log.Printf("[BillingPay] mp payment error: %v", err)
		httperr.Internal(c, "payment_creation_failed", "Erro ao criar pagamento.")
		return
	}

	resp := gin.H{
		"status":        pay.Status,
		"mp_payment_id": pay.ID,
	}

	if pay.Status == "approved" {
		if err := h.activateBarbershop(barbershopID); err != nil {
			log.Printf("[BillingPay] failed to activate barbershop %d: %v", barbershopID, err)
		}
	}

	if pay.Status == "pending" || pay.Status == "in_process" {
		resp["qr_code"] = pay.PointOfInteraction.TransactionData.QRCode
		resp["qr_code_base64"] = pay.PointOfInteraction.TransactionData.QRCodeBase64
	}

	c.JSON(http.StatusOK, resp)
}

// activateBarbershop sets status=active and extends subscription by 1 month.
func (h *BillingHandler) activateBarbershop(barbershopID uint) error {
	var shop models.Barbershop
	if err := h.db.Select("id, subscription_expires_at").First(&shop, barbershopID).Error; err != nil {
		return err
	}

	now := time.Now()
	base := now
	if shop.SubscriptionExpiresAt != nil && shop.SubscriptionExpiresAt.After(now) {
		base = *shop.SubscriptionExpiresAt
	}
	expiresAt := base.AddDate(0, 1, 0)

	return h.db.Model(&models.Barbershop{}).
		Where("id = ?", barbershopID).
		Updates(map[string]interface{}{
			"status":                  "active",
			"subscription_expires_at": expiresAt,
		}).Error
}
