package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

type PaymentHandler struct {
	listPayments *ucPayment.ListPaymentsForBarbershop
}

func NewPaymentHandler(
	listPayments *ucPayment.ListPaymentsForBarbershop,
) *PaymentHandler {
	return &PaymentHandler{
		listPayments: listPayments,
	}
}

// ======================================================
// GET /api/me/payments
// ======================================================
func (h *PaymentHandler) List(c *gin.Context) {

	// --------------------------------------------------
	// 1️⃣ Contexto do auth middleware (defensivo)
	// --------------------------------------------------
	barbershopIDAny, exists := c.Get("barbershopID")
	if !exists {
		httperr.Unauthorized(
			c,
			"unauthorized",
			"Acesso não autorizado.",
		)
		return
	}

	barbershopID, ok := barbershopIDAny.(uint)
	if !ok || barbershopID == 0 {
		httperr.Unauthorized(
			c,
			"unauthorized",
			"Acesso não autorizado.",
		)
		return
	}

	// --------------------------------------------------
	// 2️⃣ Query params
	// --------------------------------------------------
	var (
		status    *string
		startDate *time.Time
		endDate   *time.Time
	)

	if v := c.Query("status"); v != "" {
		status = &v
	}

	if v := c.Query("start_date"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			httperr.BadRequest(
				c,
				"invalid_start_date",
				"Formato inválido para start_date (YYYY-MM-DD).",
			)
			return
		}
		startDate = &t
	}

	if v := c.Query("end_date"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			httperr.BadRequest(
				c,
				"invalid_end_date",
				"Formato inválido para end_date (YYYY-MM-DD).",
			)
			return
		}
		endDate = &t
	}

	// --------------------------------------------------
	// 3️⃣ Use case
	// --------------------------------------------------
	payments, err := h.listPayments.Execute(
		c.Request.Context(),
		ucPayment.ListPaymentsInput{
			BarbershopID: barbershopID,
			Status:       status,
			StartDate:    startDate,
			EndDate:      endDate,
		},
	)

	if err != nil {
		httperr.Internal(
			c,
			"list_payments_failed",
			"Erro ao listar pagamentos.",
		)
		return
	}

	// --------------------------------------------------
	// 4️⃣ Response
	// --------------------------------------------------
	c.JSON(http.StatusOK, gin.H{
		"data":  payments,
		"total": len(payments),
	})
}
