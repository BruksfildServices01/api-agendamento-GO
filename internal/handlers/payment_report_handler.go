package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	uc "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

type PaymentReportHandler struct {
	uc *uc.GetPaymentSummary
}

func NewPaymentReportHandler(
	uc *uc.GetPaymentSummary,
) *PaymentReportHandler {
	return &PaymentReportHandler{uc: uc}
}

func (h *PaymentReportHandler) Summary(c *gin.Context) {

	barbershopID := c.GetUint(middleware.ContextBarbershopID)

	var from *time.Time
	var to *time.Time

	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = &t
		}
	}

	summary, err := h.uc.Execute(
		c.Request.Context(),
		barbershopID,
		from,
		to,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed_to_generate_report",
		})
		return
	}

	c.JSON(http.StatusOK, summary)
}
