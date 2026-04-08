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

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

// MPWebhookHandler processa as notificações IPN do Mercado Pago.
// O MP envia apenas o tipo e o ID do pagamento; o handler busca
// os detalhes via API para extrair o external_reference e o status.
type MPWebhookHandler struct {
	markMPPaid        *ucPayment.MarkMPPaymentAsPaid
	globalAccessToken string
	db                *gorm.DB
}

func NewMPWebhookHandler(
	markMPPaid *ucPayment.MarkMPPaymentAsPaid,
	globalAccessToken string,
	db *gorm.DB,
) *MPWebhookHandler {
	return &MPWebhookHandler{
		markMPPaid:        markMPPaid,
		globalAccessToken: globalAccessToken,
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
		Where("tx_id = ?", txid).
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
