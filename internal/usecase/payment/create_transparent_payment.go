package payment

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domainAppointment "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	domainNotification "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	domainTicket "github.com/BruksfildServices01/barber-scheduler/internal/domain/ticket"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

const mpPayPrefix = "mp_pay:"

// CreateTransparentPayment cria um pagamento via Checkout Transparente do Mercado Pago.
// Suporta PIX, cartão de crédito e cartão de débito.
// É idempotente: se já existir um pagamento MP criado para este agendamento, reutiliza.
type CreateTransparentPayment struct {
	repo         domain.Repository
	gateway      domain.TransparentGateway
	audit        *audit.Dispatcher
	backURL      string
	db           *gorm.DB
	apptNotifier domainNotification.AppointmentNotifier
	ticketRepo   domainTicket.Repository
	appURL       string
}

func NewCreateTransparentPayment(
	repo domain.Repository,
	gateway domain.TransparentGateway,
	audit *audit.Dispatcher,
	backURL string,
	db *gorm.DB,
	apptNotifier domainNotification.AppointmentNotifier,
	ticketRepo domainTicket.Repository,
	appURL string,
) *CreateTransparentPayment {
	return &CreateTransparentPayment{
		repo:         repo,
		gateway:      gateway,
		audit:        audit,
		backURL:      backURL,
		db:           db,
		apptNotifier: apptNotifier,
		ticketRepo:   ticketRepo,
		appURL:       appURL,
	}
}

// TransparentPaymentInput agrupa os dados enviados pelo frontend.
type TransparentPaymentInput struct {
	BarbershopID    uint
	AppointmentID   uint
	PayerEmail      string
	PayerCPF        string
	PaymentMethodID string // "pix", "visa", "master", "elo", "amex", "debelo"
	Token           string // token do cartão (vazio para PIX)
	Installments    int    // 1 para PIX e débito
	// Opcional: quando há um pedido (produtos) associado ao agendamento.
	// O valor do pedido é somado ao valor do agendamento no pagamento.
	OrderID          *uint
	OrderAmountCents int64
}

func (uc *CreateTransparentPayment) Execute(
	ctx context.Context,
	input TransparentPaymentInput,
	gatewayOverride ...domain.TransparentGateway,
) (*models.Payment, *domain.TransparentPaymentResult, error) {
	gateway := uc.gateway
	if len(gatewayOverride) > 0 && gatewayOverride[0] != nil {
		gateway = gatewayOverride[0]
	}

	// ==================================================
	// 1) BEGIN TX
	// ==================================================
	tx, err := uc.repo.BeginTx(ctx, input.BarbershopID)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	// ==================================================
	// 2) Lock do payment por appointment (FOR UPDATE)
	// ==================================================
	payment, err := tx.GetByAppointmentIDForUpdate(ctx, input.BarbershopID, input.AppointmentID)
	if err != nil {
		return nil, nil, err
	}
	if payment == nil {
		return nil, nil, httperr.ErrBusiness("payment_not_found")
	}

	// ==================================================
	// 3) Idempotência: já existe um pagamento MP criado
	// ==================================================
	if payment.TxID != nil && strings.HasPrefix(*payment.TxID, mpPayPrefix) {
		mpPaymentID, _ := strconv.ParseInt(strings.TrimPrefix(*payment.TxID, mpPayPrefix), 10, 64)
		qrCode := ""
		if payment.QRCode != nil {
			qrCode = *payment.QRCode
		}
		if err := tx.Commit(); err != nil {
			return nil, nil, err
		}
		return payment, &domain.TransparentPaymentResult{
			MPPaymentID:  mpPaymentID,
			Status:       string(payment.Status),
			QRCode:       qrCode,
		}, nil
	}

	// ==================================================
	// 4) Validações + ajuste de valor combinado (serviço + pedido)
	// ==================================================
	if domain.Status(payment.Status) != domain.StatusPending {
		return nil, nil, httperr.ErrBusiness("payment_not_pending")
	}
	if input.PayerEmail == "" {
		return nil, nil, httperr.ErrBusiness("payer_email_required")
	}

	// Se há um pedido associado, combina o valor e vincula via BundledOrderID.
	// Idempotente: só atualiza se ainda não vinculado.
	if input.OrderID != nil && input.OrderAmountCents > 0 && payment.BundledOrderID == nil {
		payment.Amount += input.OrderAmountCents
		payment.BundledOrderID = input.OrderID
		if err := tx.UpdatePaymentTx(ctx, input.BarbershopID, payment); err != nil {
			return nil, nil, fmt.Errorf("failed to update payment with order amount: %w", err)
		}
	}

	if payment.Amount < 100 {
		return nil, nil, httperr.ErrBusiness("invalid_amount")
	}

	installments := input.Installments
	if installments <= 0 {
		installments = 1
	}

	// ==================================================
	// 5) Criar pagamento no Mercado Pago
	// ==================================================
	externalReference := strconv.FormatUint(uint64(payment.ID), 10)
	description := fmt.Sprintf("Agendamento #%d", input.AppointmentID)

	// Notification URL só é enviada quando a URL do backend é pública (não localhost).
	// O MP rejeita URLs localhost/127.0.0.1 como notification_url inválida.
	notificationURL := ""
	if !strings.Contains(uc.backURL, "localhost") && !strings.Contains(uc.backURL, "127.0.0.1") {
		notificationURL = fmt.Sprintf("%s/api/webhooks/mp", uc.backURL)
	}

	result, err := gateway.CreatePayment(domain.TransparentPaymentInput{
		AmountCents:       payment.Amount,
		Description:       description,
		ExternalReference: externalReference,
		NotificationURL:   notificationURL,
		PayerEmail:        input.PayerEmail,
		PayerCPF:          input.PayerCPF,
		PaymentMethodID:   input.PaymentMethodID,
		Token:             input.Token,
		Installments:      installments,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("mp create payment failed: %w", err)
	}

	// ==================================================
	// 6) Persistir TxID e QR code
	// ==================================================
	txid := mpPayPrefix + strconv.FormatInt(result.MPPaymentID, 10)
	payment.TxID = &txid
	if result.QRCode != "" {
		payment.QRCode = &result.QRCode
	}

	// ==================================================
	// 7) Se aprovado imediatamente (cartão), marcar como pago
	// ==================================================
	if result.Status == "approved" {
		now := time.Now().UTC()
		payment.Status = models.PaymentStatus(domain.StatusPaid)
		payment.PaidAt = &now

		if err := tx.MarkAsPaid(ctx, input.BarbershopID, payment); err != nil {
			return nil, nil, fmt.Errorf("failed to mark payment as paid: %w", err)
		}
		if err := tx.RegisterEvent(ctx, txid, mpPaidEvent); err != nil {
			return nil, nil, fmt.Errorf("failed to register mp event: %w", err)
		}

		if payment.AppointmentID != nil {
			ap, err := tx.GetAppointmentForUpdate(ctx, input.BarbershopID, *payment.AppointmentID)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to lock appointment: %w", err)
			}
			if ap != nil && ap.Status == models.AppointmentStatus(domainAppointment.StatusAwaitingPayment) {
				ap.Status = models.AppointmentStatus(domainAppointment.StatusScheduled)
				if err := tx.UpdateAppointmentTx(ctx, ap); err != nil {
					return nil, nil, fmt.Errorf("failed to update appointment: %w", err)
				}
			}
		}

		// Se há um pedido vinculado, marcar como pago e dar baixa no estoque.
		if payment.BundledOrderID != nil {
			order, err := tx.GetOrderForUpdate(ctx, input.BarbershopID, *payment.BundledOrderID)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to lock order: %w", err)
			}
			if order != nil && order.Status == models.OrderStatusPending {
				items, err := tx.ListOrderItems(ctx, input.BarbershopID, order.ID)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to list order items: %w", err)
				}
				for _, it := range items {
					if err := tx.DecreaseProductStock(ctx, input.BarbershopID, it.ProductID, it.Quantity); err != nil {
						return nil, nil, fmt.Errorf("failed to decrease stock: %w", err)
					}
				}
				order.Status = models.OrderStatusPaid
				if err := tx.UpdateOrderTx(ctx, order); err != nil {
					return nil, nil, fmt.Errorf("failed to update order: %w", err)
				}
			}
		}
	} else {
		if err := tx.UpdatePaymentTx(ctx, input.BarbershopID, payment); err != nil {
			return nil, nil, fmt.Errorf("failed to update payment: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit failed: %w", err)
	}

	uc.audit.Dispatch(audit.Event{
		BarbershopID: input.BarbershopID,
		Action:       "payment_transparent_created",
		Entity:       "payment",
		EntityID:     &payment.ID,
		Metadata: map[string]any{
			"mp_payment_id":     result.MPPaymentID,
			"payment_method_id": input.PaymentMethodID,
			"status":            result.Status,
		},
	})

	// Send confirmation email only when payment is immediately approved (card).
	if result.Status == "approved" && payment.AppointmentID != nil &&
		uc.apptNotifier != nil && uc.db != nil {
		sendAppointmentConfirmedEmail(ctx, uc.db, uc.apptNotifier, uc.ticketRepo, uc.appURL, *payment.AppointmentID)
	}

	return payment, result, nil
}
