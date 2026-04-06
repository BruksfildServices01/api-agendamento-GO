package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mercadopago/sdk-go/pkg/config"
	"github.com/mercadopago/sdk-go/pkg/payment"

	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

// MPWebhookHandler processa as notificações IPN do Mercado Pago.
// O MP envia apenas o tipo e o ID do pagamento; o handler busca
// os detalhes via API para extrair o external_reference e o status.
type MPWebhookHandler struct {
	markMPPaid    *ucPayment.MarkMPPaymentAsPaid
	paymentClient payment.Client
}

func NewMPWebhookHandler(
	markMPPaid *ucPayment.MarkMPPaymentAsPaid,
	accessToken string,
) *MPWebhookHandler {
	var paymentClient payment.Client
	if accessToken != "" {
		cfg, err := config.New(accessToken)
		if err == nil {
			paymentClient = payment.NewClient(cfg)
		}
	}
	return &MPWebhookHandler{
		markMPPaid:    markMPPaid,
		paymentClient: paymentClient,
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
// POST /api/webhooks/mp
func (h *MPWebhookHandler) Handle(c *gin.Context) {
	var notif mpNotification
	if err := c.ShouldBindJSON(&notif); err != nil {
		c.Status(http.StatusOK) // webhook nunca retorna erro
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

func (h *MPWebhookHandler) processPayment(ctx context.Context, mpPaymentIDStr string) error {
	if h.paymentClient == nil {
		return fmt.Errorf("MP payment client not configured")
	}

	mpPaymentID, err := strconv.Atoi(mpPaymentIDStr)
	if err != nil {
		return fmt.Errorf("invalid payment id %q: %w", mpPaymentIDStr, err)
	}

	resp, err := h.paymentClient.Get(ctx, mpPaymentID)
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
