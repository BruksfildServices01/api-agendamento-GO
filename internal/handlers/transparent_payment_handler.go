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

// TransparentPaymentHandler lida com pagamentos via Checkout Transparente
// (PIX, cartão de crédito e débito).
type TransparentPaymentHandler struct {
	db                          *gorm.DB
	createPaymentForAppointment *ucPayment.CreatePaymentForAppointment
	createTransparentPayment    *ucPayment.CreateTransparentPayment
}

func NewTransparentPaymentHandler(
	db *gorm.DB,
	createPaymentForAppointment *ucPayment.CreatePaymentForAppointment,
	createTransparentPayment *ucPayment.CreateTransparentPayment,
) *TransparentPaymentHandler {
	return &TransparentPaymentHandler{
		db:                          db,
		createPaymentForAppointment: createPaymentForAppointment,
		createTransparentPayment:    createTransparentPayment,
	}
}

type transparentPaymentRequest struct {
	PayerEmail      string `json:"payer_email"      binding:"required,email"`
	PayerCPF        string `json:"payer_cpf"`
	PaymentMethodID string `json:"payment_method_id" binding:"required"`
	Token           string `json:"token"`
	Installments    int    `json:"installments"`
}

type transparentPaymentResponse struct {
	PaymentID    uint   `json:"payment_id"`
	MPPaymentID  int64  `json:"mp_payment_id"`
	Status       string `json:"status"`
	StatusDetail string `json:"status_detail,omitempty"`
	// PIX
	QRCode       string `json:"qr_code,omitempty"`
	QRCodeBase64 string `json:"qr_code_base64,omitempty"`
	TicketURL    string `json:"ticket_url,omitempty"`
}

// POST /api/public/:slug/appointments/:id/payment/transparent
func (h *TransparentPaymentHandler) CreatePayment(c *gin.Context) {
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

	var req transparentPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_body", err.Error())
		return
	}

	ctx := c.Request.Context()

	// Garante que existe um registro de pagamento para o agendamento
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
		httperr.BadRequest(c, "appointment_not_awaiting_payment", "Este agendamento não requer pagamento.")
		return
	}

	if h.createPaymentForAppointment != nil {
		if _, err := h.createPaymentForAppointment.Execute(ctx, &ap); err != nil {
			httperr.Internal(c, "failed_to_prepare_payment", "Erro ao preparar pagamento.")
			return
		}
	}

	payment, result, err := h.createTransparentPayment.Execute(ctx, ucPayment.TransparentPaymentInput{
		BarbershopID:    shop.ID,
		AppointmentID:   uint(appointmentID),
		PayerEmail:      req.PayerEmail,
		PayerCPF:        req.PayerCPF,
		PaymentMethodID: req.PaymentMethodID,
		Token:           req.Token,
		Installments:    req.Installments,
	})
	if err != nil {
		switch {
		case httperr.IsBusiness(err, "payment_not_found"):
			httperr.BadRequest(c, "payment_not_found", "Pagamento não encontrado.")
		case httperr.IsBusiness(err, "payment_not_pending"):
			httperr.BadRequest(c, "payment_not_pending", "Pagamento não está pendente.")
		case httperr.IsBusiness(err, "invalid_amount"):
			httperr.BadRequest(c, "invalid_amount", "Valor inválido para pagamento.")
		case httperr.IsBusiness(err, "payer_email_required"):
			httperr.BadRequest(c, "payer_email_required", "E-mail do pagador é obrigatório.")
		default:
			log.Printf("[TRANSPARENT] CreatePayment error slug=%s appointment=%d: %v", slug, appointmentID, err)
			httperr.Internal(c, "payment_creation_failed", "Erro ao criar pagamento.")
		}
		return
	}

	c.JSON(http.StatusCreated, transparentPaymentResponse{
		PaymentID:    payment.ID,
		MPPaymentID:  result.MPPaymentID,
		Status:       result.Status,
		StatusDetail: result.StatusDetail,
		QRCode:       result.QRCode,
		QRCodeBase64: result.QRCodeBase64,
		TicketURL:    result.TicketURL,
	})
}
