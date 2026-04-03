package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	ucAppointment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/appointment"
)

type ClosureAdjustmentHandler struct {
	uc *ucAppointment.AdjustClosure
}

func NewClosureAdjustmentHandler(uc *ucAppointment.AdjustClosure) *ClosureAdjustmentHandler {
	return &ClosureAdjustmentHandler{uc: uc}
}

type closureAdjustmentRequest struct {
	DeltaFinalAmountCents *int64  `json:"delta_final_amount_cents"`
	DeltaPaymentMethod    *string `json:"delta_payment_method"`
	DeltaOperationalNote  *string `json:"delta_operational_note"`
	Reason                string  `json:"reason" binding:"required"`
}

// Create handles POST /api/me/appointments/:id/closure/adjustment
func (h *ClosureAdjustmentHandler) Create(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	barberID := c.MustGet(middleware.ContextUserID).(uint)

	appointmentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid appointment id"})
		return
	}

	var req closureAdjustmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}

	adjustment, err := h.uc.Execute(c.Request.Context(), ucAppointment.AdjustClosureInput{
		BarbershopID:          barbershopID,
		BarberID:              barberID,
		AppointmentID:         uint(appointmentID),
		DeltaFinalAmountCents: req.DeltaFinalAmountCents,
		DeltaPaymentMethod:    req.DeltaPaymentMethod,
		DeltaOperationalNote:  req.DeltaOperationalNote,
		Reason:                req.Reason,
	})
	if err != nil {
		switch {
		case errors.Is(err, ucAppointment.ErrClosureNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "closure_not_found"})
		case errors.Is(err, ucAppointment.ErrAdjustmentWindowExpired):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "adjustment_window_expired"})
		case errors.Is(err, ucAppointment.ErrNoAdjustmentFields):
			c.JSON(http.StatusBadRequest, gin.H{"error": "no_adjustment_fields"})
		case httperr.IsBusiness(err, "invalid_final_amount"):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_final_amount"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusCreated, adjustment)
}
