package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

type PixWebhookHandler struct {
	markPaid *payment.MarkPaymentAsPaid
}

func NewPixWebhookHandler(
	markPaid *payment.MarkPaymentAsPaid,
) *PixWebhookHandler {
	return &PixWebhookHandler{
		markPaid: markPaid,
	}
}

type PixWebhookPayload struct {
	TxID  string `json:"txid"`
	Event string `json:"event"`
}

func (h *PixWebhookHandler) Handle(c *gin.Context) {
	var payload PixWebhookPayload

	// --------------------------------------------------
	// 1️⃣ Decode tolerante (webhook nunca falha)
	// --------------------------------------------------
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.Status(http.StatusOK)
		return
	}

	// --------------------------------------------------
	// 2️⃣ Validação mínima
	// --------------------------------------------------
	txid := strings.TrimSpace(payload.TxID)
	if txid == "" {
		c.Status(http.StatusOK)
		return
	}

	if payload.Event != "paid" {
		c.Status(http.StatusOK)
		return
	}

	// --------------------------------------------------
	// 3️⃣ Execução idempotente
	// --------------------------------------------------
	_ = h.markPaid.Execute(
		c.Request.Context(),
		txid,
	)

	// --------------------------------------------------
	// 4️⃣ Webhook SEMPRE responde 200
	// --------------------------------------------------
	c.Status(http.StatusOK)
}
