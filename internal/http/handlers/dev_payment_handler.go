package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

// DevPaymentHandler expõe endpoints de apoio para testes em modo mock.
// NUNCA registrar em produção (MPProvider == "mp").
type DevPaymentHandler struct {
	markPaid *ucPayment.MarkMPPaymentAsPaid
}

func NewDevPaymentHandler(markPaid *ucPayment.MarkMPPaymentAsPaid) *DevPaymentHandler {
	return &DevPaymentHandler{markPaid: markPaid}
}

// ConfirmPayment simula a confirmação de pagamento que normalmente viria do webhook do MP.
// POST /api/dev/payments/:id/confirm
//
// Uso em testes: crie um appointment que exija pagamento, gere o payment via
// /appointments/:id/payment/transparent ou /payment/mp, anote o payment_id
// retornado e chame este endpoint para confirmar sem precisar de credenciais MP reais.
func (h *DevPaymentHandler) ConfirmPayment(c *gin.Context) {
	paymentIDStr := c.Param("id")
	paymentID, err := strconv.ParseUint(paymentIDStr, 10, 64)
	if err != nil || paymentID == 0 {
		httperr.BadRequest(c, "invalid_payment_id", "ID de pagamento inválido.")
		return
	}

	// Usa um mpPaymentID mock único por pagamento para garantir idempotência.
	mockMPID := fmt.Sprintf("mock-%d", paymentID)

	if err := h.markPaid.Execute(
		c.Request.Context(),
		fmt.Sprintf("%d", paymentID),
		mockMPID,
	); err != nil {
		httperr.Internal(c, "confirm_failed", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":         true,
		"payment_id": paymentID,
		"mock_mp_id": mockMPID,
	})
}
