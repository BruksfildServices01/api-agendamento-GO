package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mercadopago/sdk-go/pkg/config"
	"github.com/mercadopago/sdk-go/pkg/payment"
	"gorm.io/gorm"

	infraMP "github.com/BruksfildServices01/barber-scheduler/internal/infra/mp"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

// MPWebhookHandler processa as notificações IPN do Mercado Pago.
// O MP envia apenas o tipo e o ID do pagamento; o handler busca
// os detalhes via API para extrair o external_reference e o status.
type MPWebhookHandler struct {
	markMPPaid        *ucPayment.MarkMPPaymentAsPaid
	globalAccessToken string
	webhookSecret     string
	// requireSignature=true quando MPProvider=="mp".
	// Se true e webhookSecret vazio, o webhook é rejeitado sem processar.
	requireSignature bool
	db               *gorm.DB
}

func NewMPWebhookHandler(
	markMPPaid *ucPayment.MarkMPPaymentAsPaid,
	globalAccessToken string,
	webhookSecret string,
	requireSignature bool,
	db *gorm.DB,
) *MPWebhookHandler {
	return &MPWebhookHandler{
		markMPPaid:        markMPPaid,
		globalAccessToken: globalAccessToken,
		webhookSecret:     webhookSecret,
		requireSignature:  requireSignature,
		db:                db,
	}
}

// mpNotification é o corpo do IPN enviado pelo Mercado Pago.
type mpNotification struct {
	Action string `json:"action"`
	Type   string `json:"type"`
	Data   struct {
		ID string `json:"id"`
	} `json:"data"`
}

// Handle processa a notificação IPN do MP.
// POST /api/webhooks/mp  ou  POST /webhooks/mp
func (h *MPWebhookHandler) Handle(c *gin.Context) {
	var notif mpNotification
	if err := c.ShouldBindJSON(&notif); err != nil {
		c.Status(http.StatusOK)
		return
	}

	if notif.Type != "payment" || notif.Data.ID == "" {
		c.Status(http.StatusOK)
		return
	}

	// Valida assinatura HMAC.
	if h.webhookSecret != "" {
		xSig := c.GetHeader("x-signature")
		xReqID := c.GetHeader("x-request-id")
		if !infraMP.VerifyWebhookSignature(h.webhookSecret, xSig, xReqID, notif.Data.ID) {
			log.Printf("[MP WEBHOOK] assinatura inválida para pagamento %s — ignorado", notif.Data.ID)
			c.Status(http.StatusOK) // 200 para o MP não retentar
			return
		}
	} else if h.requireSignature {
		// Produção (requireSignature=true) sem secret configurado.
		// Bloqueia sem processar para evitar fraude por webhook forjado.
		log.Printf("[MP WEBHOOK] ALERTA: MP_WEBHOOK_SECRET não configurado em modo produção — webhook rejeitado sem processar")
		c.Status(http.StatusOK) // 200 para o MP não retentar
		return
	} else {
		log.Printf("[MP WEBHOOK] MP_WEBHOOK_SECRET não configurado — validação ignorada (modo dev)")
	}

	mpPaymentID := notif.Data.ID

	go func() {
		if err := h.processPayment(context.Background(), mpPaymentID); err != nil {
			log.Printf("[MP WEBHOOK] error processing payment %s: %v", mpPaymentID, err)
		}
	}()

	c.Status(http.StatusOK)
}

// resolveAccessToken descobre qual access token usar para buscar o pagamento no MP.
// Primeiro tenta encontrar o pagamento no banco pelo TxID e pegar o token da barbearia.
// Se não achar ou a barbearia não tiver token próprio, usa o token global.
func (h *MPWebhookHandler) resolveAccessToken(ctx context.Context, mpPaymentIDStr string) string {
	txid := "mp_pay:" + mpPaymentIDStr

	var p models.Payment
	err := h.db.WithContext(ctx).
		Where("txid = ?", txid).
		Select("barbershop_id").
		First(&p).Error

	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[MP WEBHOOK] resolveAccessToken DB error: %v", err)
		}
		return h.globalAccessToken
	}

	var cfg models.BarbershopPaymentConfig
	err = h.db.WithContext(ctx).
		Where("barbershop_id = ?", p.BarbershopID).
		Select("mp_access_token").
		First(&cfg).Error

	if err != nil || strings.TrimSpace(cfg.MPAccessToken) == "" {
		return h.globalAccessToken
	}

	return cfg.MPAccessToken
}

// CheckPaymentStatus é chamado pelo frontend como fallback quando o webhook demora.
// GET /api/public/:slug/appointments/:id/payment/status
func (h *MPWebhookHandler) CheckPaymentStatus(c *gin.Context) {
	appointmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil || appointmentID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_appointment_id"})
		return
	}

	slug := c.Param("slug")
	var shop models.Barbershop
	if err := h.db.WithContext(c.Request.Context()).
		Where("slug = ?", slug).Select("id").First(&shop).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "not_found"})
		return
	}

	var p models.Payment
	if err := h.db.WithContext(c.Request.Context()).
		Where("barbershop_id = ? AND appointment_id = ?", shop.ID, uint(appointmentID)).
		First(&p).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "no_payment"})
		return
	}

	if p.Status == "paid" {
		c.JSON(http.StatusOK, gin.H{"status": "confirmed"})
		return
	}

	// MPPaymentID pode ser nil — pagamentos transparentes gravam apenas TxID ("mp_pay:<id>").
	// Extrai o ID do TxID quando MPPaymentID não está preenchido.
	var mpPaymentIDStr string
	if p.MPPaymentID != nil {
		mpPaymentIDStr = strconv.FormatInt(*p.MPPaymentID, 10)
	} else if p.TxID != nil && strings.HasPrefix(*p.TxID, "mp_pay:") {
		mpPaymentIDStr = strings.TrimPrefix(*p.TxID, "mp_pay:")
	}

	if mpPaymentIDStr == "" {
		c.JSON(http.StatusOK, gin.H{"status": string(p.Status)})
		return
	}
	if err := h.processPayment(c.Request.Context(), mpPaymentIDStr); err != nil {
		log.Printf("[CheckPaymentStatus] appointment=%d mp_payment=%s error=%v", appointmentID, mpPaymentIDStr, err)
		c.JSON(http.StatusOK, gin.H{"status": string(p.Status)})
		return
	}

	// Relê o status após tentar confirmar
	var updated models.Payment
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ?", p.ID).Select("status").First(&updated).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"status": string(p.Status)})
		return
	}

	status := string(updated.Status)
	if status == "paid" {
		status = "confirmed"
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}

func (h *MPWebhookHandler) processPayment(ctx context.Context, mpPaymentIDStr string) error {
	accessToken := h.resolveAccessToken(ctx, mpPaymentIDStr)
	if accessToken == "" {
		return fmt.Errorf("no MP access token available")
	}

	cfg, err := config.New(accessToken)
	if err != nil {
		return fmt.Errorf("mp config error: %w", err)
	}
	paymentClient := payment.NewClient(cfg)

	mpPaymentID, err := strconv.Atoi(mpPaymentIDStr)
	if err != nil {
		return fmt.Errorf("invalid payment id %q: %w", mpPaymentIDStr, err)
	}

	resp, err := paymentClient.Get(ctx, mpPaymentID)
	if err != nil {
		return fmt.Errorf("failed to get MP payment: %w", err)
	}

	if resp.Status != "approved" {
		return nil
	}

	if resp.ExternalReference == "" {
		return fmt.Errorf("empty external_reference for MP payment %d", mpPaymentID)
	}

	return h.markMPPaid.Execute(ctx, resp.ExternalReference, mpPaymentIDStr)
}
