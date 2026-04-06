package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

type MPPaymentHandler struct {
	db *gorm.DB

	createPaymentForAppointment *ucPayment.CreatePaymentForAppointment
	createMPPreference          *ucPayment.CreateMPPreference
}

func NewMPPaymentHandler(
	db *gorm.DB,
	createPaymentForAppointment *ucPayment.CreatePaymentForAppointment,
	createMPPreference *ucPayment.CreateMPPreference,
) *MPPaymentHandler {
	return &MPPaymentHandler{
		db:                          db,
		createPaymentForAppointment: createPaymentForAppointment,
		createMPPreference:          createMPPreference,
	}
}

type mpPreferenceResponse struct {
	PaymentID    uint   `json:"payment_id"`
	PreferenceID string `json:"preference_id"`
	InitPoint    string `json:"init_point"`
}

// POST /api/public/:slug/appointments/:id/payment/mp
func (h *MPPaymentHandler) CreatePreference(c *gin.Context) {
	slug := c.Param("slug")

	var shop models.Barbershop
	if err := h.db.
		WithContext(c.Request.Context()).
		Where("slug = ?", slug).
		First(&shop).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
			return
		}
		httperr.Internal(c, "failed_to_load_barbershop", "Erro ao carregar barbearia.")
		return
	}

	appointmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil || appointmentID <= 0 {
		httperr.BadRequest(c, "invalid_appointment_id", "Agendamento inválido.")
		return
	}

	ctx := c.Request.Context()

	var ap models.Appointment
	if err := h.db.
		WithContext(ctx).
		Where("id = ? AND barbershop_id = ?", uint(appointmentID), shop.ID).
		First(&ap).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.NotFound(c, "appointment_not_found", "Agendamento não encontrado.")
			return
		}
		httperr.Internal(c, "failed_to_load_appointment", "Erro ao carregar agendamento.")
		return
	}

	if ap.Status != models.AppointmentStatusAwaitingPayment {
		httperr.BadRequest(
			c,
			"appointment_not_awaiting_payment",
			"Este agendamento não requer pagamento.",
		)
		return
	}

	if h.createPaymentForAppointment != nil {
		if _, err := h.createPaymentForAppointment.Execute(ctx, &ap); err != nil {
			httperr.Internal(c, "failed_to_prepare_payment", "Erro ao preparar pagamento.")
			return
		}
	}

	payment, pref, err := h.createMPPreference.Execute(
		ctx,
		shop.ID,
		uint(appointmentID),
		slug,
	)
	if err != nil {
		switch {
		case httperr.IsBusiness(err, "payment_not_found"):
			httperr.BadRequest(c, "payment_not_found", "Pagamento não encontrado.")
		case httperr.IsBusiness(err, "invalid_amount"):
			httperr.BadRequest(c, "invalid_amount", "Valor inválido para pagamento.")
		case httperr.IsBusiness(err, "payment_not_pending"):
			httperr.BadRequest(c, "payment_not_pending", "Pagamento não está pendente.")
		default:
			log.Printf("[MP] CreatePreference error slug=%s appointment=%d: %v", slug, appointmentID, err)
			httperr.Internal(c, "mp_preference_failed", "Erro ao criar preferência de pagamento.")
		}
		return
	}

	c.JSON(http.StatusCreated, mpPreferenceResponse{
		PaymentID:    payment.ID,
		PreferenceID: pref.PreferenceID,
		InitPoint:    pref.InitPoint,
	})
}
