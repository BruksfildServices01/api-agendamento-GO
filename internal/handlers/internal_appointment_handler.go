package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	ucAppointment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/appointment"
)

type InternalAppointmentHandler struct {
	createUC *ucAppointment.CreateInternalAppointment
}

func NewInternalAppointmentHandler(
	createUC *ucAppointment.CreateInternalAppointment,
) *InternalAppointmentHandler {
	return &InternalAppointmentHandler{
		createUC: createUC,
	}
}

// --------------------------------------------------
// REQUEST DTO (fluxo interno = simples)
// --------------------------------------------------

type CreateInternalAppointmentRequest struct {
	BarberID uint `json:"barber_id" binding:"required"`

	ClientName  string `json:"client_name" binding:"required"`
	ClientPhone string `json:"client_phone" binding:"required"`
	ClientEmail string `json:"client_email"`

	BarberProductID uint `json:"barber_product_id" binding:"required"`

	StartTime time.Time `json:"start_time" binding:"required"`
	EndTime   time.Time `json:"end_time" binding:"required"`

	PaymentIntent string `json:"payment_intent"` // "paid" | "pay_later"
	Notes         string `json:"notes"`
}

// --------------------------------------------------
// HANDLER
// --------------------------------------------------

func (h *InternalAppointmentHandler) Create(c *gin.Context) {

	// 1️⃣ Contexto da barbearia (middleware)
	raw, exists := c.Get(middleware.ContextBarbershopID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "barbershop context not found",
		})
		return
	}

	barbershopID, ok := raw.(uint)
	if !ok || barbershopID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid barbershop context",
		})
		return
	}

	// 2️⃣ Bind request
	var req CreateInternalAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid_request",
		})
		return
	}

	// 3️⃣ Execute use case
	appointment, err := h.createUC.Execute(
		c.Request.Context(),
		ucAppointment.CreateInternalAppointmentInput{
			BarbershopID: barbershopID,
			BarberID:     req.BarberID,

			ClientName:  req.ClientName,
			ClientPhone: req.ClientPhone,
			ClientEmail: req.ClientEmail,

			BarberProductID: req.BarberProductID,

			StartTime: req.StartTime,
			EndTime:   req.EndTime,

			PaymentIntent: req.PaymentIntent,
			Notes:         req.Notes,
		},
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 4️⃣ Response direta (sem DTO extra)
	c.JSON(http.StatusCreated, appointment)
}
