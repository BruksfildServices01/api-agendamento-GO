package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/dto"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/idempotency"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

type PublicPaymentHandler struct {
	db *gorm.DB

	// ✅ garante que existe Payment para o appointment
	createPaymentForAppointment *ucPayment.CreatePaymentForAppointment

	// ✅ gera/reusa charge PIX e atualiza payment com txid/expires_at/qr_code
	createPix *ucPayment.CreatePixPayment
}

func NewPublicPaymentHandler(
	db *gorm.DB,
	createPaymentForAppointment *ucPayment.CreatePaymentForAppointment,
	createPix *ucPayment.CreatePixPayment,
) *PublicPaymentHandler {
	return &PublicPaymentHandler{
		db:                          db,
		createPaymentForAppointment: createPaymentForAppointment,
		createPix:                   createPix,
	}
}

// ======================================================
// POST /api/public/:slug/appointments/:id/payment/pix
// ======================================================
func (h *PublicPaymentHandler) CreatePix(c *gin.Context) {
	slug := c.Param("slug")

	// --------------------------------------------------
	// 1) Resolve barbershop pelo slug
	// --------------------------------------------------
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

	// --------------------------------------------------
	// 2) Appointment ID
	// --------------------------------------------------
	appointmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil || appointmentID <= 0 {
		httperr.BadRequest(c, "invalid_appointment_id", "Agendamento inválido.")
		return
	}

	ctx := c.Request.Context()

	// --------------------------------------------------
	// 3) Carrega appointment (tenant-safe)
	// --------------------------------------------------
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
			"Este agendamento não requer pagamento PIX.",
		)
		return
	}

	// --------------------------------------------------
	// 4) Garante Payment (idempotente)
	// --------------------------------------------------
	if h.createPaymentForAppointment != nil {
		if _, err := h.createPaymentForAppointment.Execute(ctx, &ap); err != nil {
			httperr.Internal(c, "failed_to_prepare_payment", "Erro ao preparar pagamento.")
			return
		}
	}

	// --------------------------------------------------
	// 5) Cria/Reusa charge PIX (usecase já é idempotente)
	// --------------------------------------------------
	payment, pixCharge, err := h.createPix.Execute(
		ctx,
		shop.ID,
		uint(appointmentID),
	)

	if err != nil {
		switch {
		case errors.Is(err, idempotency.ErrDuplicateRequest):
			httperr.Write(c, http.StatusConflict, "duplicate_request", "Solicitação repetida. Tente novamente.")
			return

		case httperr.IsBusiness(err, "payment_not_found"):
			httperr.BadRequest(c, "payment_not_found", "Pagamento ainda não está pronto.")
			return

		case httperr.IsBusiness(err, "invalid_amount"):
			httperr.BadRequest(c, "invalid_amount", "Valor inválido para pagamento.")
			return

		case httperr.IsBusiness(err, "payment_not_pending"):
			httperr.BadRequest(c, "payment_not_pending", "Pagamento não está pendente.")
			return

		case httperr.IsBusiness(err, "payment_inconsistent_state"):
			httperr.Internal(c, "payment_inconsistent_state", "Pagamento em estado inconsistente.")
			return

		default:
			httperr.Internal(c, "payment_creation_failed", "Erro ao gerar pagamento.")
			return
		}
	}

	// --------------------------------------------------
	// 6) Response padronizada
	// --------------------------------------------------
	c.JSON(http.StatusCreated, dto.PixResponse{
		PaymentID: payment.ID,
		Pix: dto.PixPayload{
			TxID:      pixCharge.TxID,
			QRCode:    pixCharge.QRCode,
			ExpiresAt: pixCharge.ExpiresAt,
		},
	})
}
