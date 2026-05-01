package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/apperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/http/httperr"
	paymentinfra "github.com/BruksfildServices01/barber-scheduler/internal/infra/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

// TransparentPaymentHandler lida com pagamentos via Checkout Transparente
// (PIX, cartão de crédito e débito).
type TransparentPaymentHandler struct {
	db                          *gorm.DB
	createPaymentForAppointment *ucPayment.CreatePaymentForAppointment
	createTransparentPayment    *ucPayment.CreateTransparentPayment
	registry                    *paymentinfra.ProviderRegistry
}

func NewTransparentPaymentHandler(
	db *gorm.DB,
	createPaymentForAppointment *ucPayment.CreatePaymentForAppointment,
	createTransparentPayment *ucPayment.CreateTransparentPayment,
	registry *paymentinfra.ProviderRegistry,
) *TransparentPaymentHandler {
	return &TransparentPaymentHandler{
		db:                          db,
		createPaymentForAppointment: createPaymentForAppointment,
		createTransparentPayment:    createTransparentPayment,
		registry:                    registry,
	}
}

type transparentPaymentRequest struct {
	PayerEmail      string `json:"payer_email"      binding:"required,email"`
	PayerCPF        string `json:"payer_cpf"`
	PaymentMethodID string `json:"payment_method_id" binding:"required"`
	Token           string `json:"token"`
	Installments    int    `json:"installments"`
	// Opcional: pedido de produtos a ser cobrado junto com o agendamento.
	OrderID          *uint `json:"order_id,omitempty"`
	OrderAmountCents int64 `json:"order_amount_cents,omitempty"`
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

func mapTransparentPaymentError(c *gin.Context, slug string, appointmentID int, err error) {
	switch {
	case apperr.IsBusiness(err, "payment_not_found"):
		httperr.BadRequest(c, "payment_not_found", "Pagamento não encontrado.")
	case apperr.IsBusiness(err, "payment_not_pending"):
		httperr.BadRequest(c, "payment_not_pending", "Pagamento não está pendente.")
	case apperr.IsBusiness(err, "invalid_amount"):
		httperr.BadRequest(c, "invalid_amount", "Valor inválido para pagamento.")
	case apperr.IsBusiness(err, "payer_email_required"):
		httperr.BadRequest(c, "payer_email_required", "E-mail do pagador é obrigatório.")
	default:
		log.Printf("[TRANSPARENT] CreatePayment error slug=%s appointment=%d: %v", slug, appointmentID, err)
		httperr.Internal(c, "payment_creation_failed", "Erro ao criar pagamento.")
	}
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

	// Normaliza installments: 0 ou não informado → 1 (padrão à vista).
	// PIX não usa installments, mas normalizamos para evitar enviar 0 ao MP.
	if req.Installments <= 0 {
		req.Installments = 1
	}

	// Para cartão de crédito/débito, installments deve ser 1–12.
	// Para PIX, o campo não tem significado prático mas o range ainda vale.
	if req.Installments > 12 {
		httperr.BadRequest(c, "invalid_installments", "Número de parcelas deve ser entre 1 e 12.")
		return
	}

	// Validação defensiva de valores monetários do pedido.
	// O valor final do serviço vem sempre do banco (via payment record).
	// order_amount_cents é o valor adicional de produtos — não pode ser negativo.
	if req.OrderAmountCents < 0 {
		httperr.BadRequest(c, "invalid_order_amount", "Valor do pedido não pode ser negativo.")
		return
	}
	// Limite defensivo: R$ 50.000,00 por transação
	const maxTransactionCents int64 = 5_000_000
	if req.OrderAmountCents > maxTransactionCents {
		httperr.BadRequest(c, "invalid_order_amount", "Valor do pedido excede o limite permitido.")
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

	// Carrega configuração de pagamento da barbearia
	var paymentCfg models.BarbershopPaymentConfig
	hasCfg := h.db.WithContext(ctx).Where("barbershop_id = ?", shop.ID).First(&paymentCfg).Error == nil

	// Bloqueia se as credenciais MP não estão configuradas
	if !hasCfg || paymentCfg.MPAccessToken == "" || paymentCfg.MPPublicKey == "" {
		httperr.BadRequest(c, "payment_not_configured", "Esta barbearia ainda não configurou o pagamento online.")
		return
	}

	if hasCfg {
		// Validar se o método de pagamento está habilitado
		method := req.PaymentMethodID
		var blocked bool
		switch {
		case method == "pix" && !paymentCfg.AcceptPix:
			blocked = true
		case method != "pix" && isDebitMethod(method) && !paymentCfg.AcceptDebit:
			blocked = true
		case method != "pix" && !isDebitMethod(method) && !paymentCfg.AcceptCredit:
			blocked = true
		}
		if blocked {
			httperr.BadRequest(c, "payment_method_not_accepted", "Esta forma de pagamento não é aceita por esta barbearia.")
			return
		}
	}

	// Obtém o gateway do provider configurado para esta barbearia.
	// O registry é o único lugar que conhece qual provider usar e como instanciá-lo.
	gw, err := h.registry.TransparentGatewayFor(ctx, paymentCfg)
	if err != nil {
		log.Printf("[TRANSPARENT] gateway error barbershop=%d: %v", shop.ID, err)
		httperr.Internal(c, "payment_gateway_error", "Erro ao inicializar gateway de pagamento.")
		return
	}

	payment, result, err := h.createTransparentPayment.Execute(ctx, ucPayment.TransparentPaymentInput{
		BarbershopID:     shop.ID,
		AppointmentID:    uint(appointmentID),
		PayerEmail:       req.PayerEmail,
		PayerCPF:         req.PayerCPF,
		PaymentMethodID:  req.PaymentMethodID,
		Token:            req.Token,
		Installments:     req.Installments,
		OrderID:          req.OrderID,
		OrderAmountCents: req.OrderAmountCents,
	}, gw)
	if err != nil {
		mapTransparentPaymentError(c, slug, appointmentID, err)
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

// isDebitMethod returns true for known debit card payment_method_ids.
func isDebitMethod(method string) bool {
	switch method {
	case "debvisa", "debmaster", "debelo", "debcabal", "redcompra":
		return true
	}
	return false
}
