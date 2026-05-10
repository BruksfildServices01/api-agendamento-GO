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
	"github.com/BruksfildServices01/barber-scheduler/internal/apperr"
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
		return nil, nil, apperr.ErrBusiness("payment_not_found")
	}

	// ==================================================
	// 3) Idempotência: já existe um pagamento criado por qualquer provider
	// ==================================================
	// Detecta pagamento já processado por dois caminhos:
	//   a) provider_payment_id preenchido — qualquer provider (MP novo, PagBank, etc.)
	//   b) TxID com prefixo "mp_pay:" — legado MP para payments antigos sem provider_payment_id
	// Isso evita dupla cobrança em retries, timeouts ou duplo clique no mobile.
	alreadyProcessed := (payment.ProviderPaymentID != nil && *payment.ProviderPaymentID != "") ||
		(payment.TxID != nil && strings.HasPrefix(*payment.TxID, mpPayPrefix))

	if alreadyProcessed {
		qrCode := ""
		if payment.QRCode != nil {
			qrCode = *payment.QRCode
		}
		providerPaymentID := ""
		if payment.ProviderPaymentID != nil {
			providerPaymentID = *payment.ProviderPaymentID
		}
		var mpPaymentID int64
		if payment.TxID != nil && strings.HasPrefix(*payment.TxID, mpPayPrefix) {
			mpPaymentID, _ = strconv.ParseInt(strings.TrimPrefix(*payment.TxID, mpPayPrefix), 10, 64)
		}
		if err := tx.Commit(); err != nil {
			return nil, nil, err
		}
		return payment, &domain.TransparentPaymentResult{
			ProviderPaymentID: providerPaymentID,
			MPPaymentID:       mpPaymentID,
			Status:            string(payment.Status),
			QRCode:            qrCode,
		}, nil
	}

	// ==================================================
	// 4) Validações + ajuste de valor combinado (serviço + pedido)
	// ==================================================
	if domain.Status(payment.Status) != domain.StatusPending {
		return nil, nil, apperr.ErrBusiness("payment_not_pending")
	}
	if input.PayerEmail == "" {
		return nil, nil, apperr.ErrBusiness("payer_email_required")
	}

	// Se há um pedido associado, combina o valor e vincula via BundledOrderID.
	// Idempotente: só atualiza se ainda não vinculado.
	//
	// SEGURANÇA: o valor do pedido é lido do banco (order.TotalAmount), nunca do frontend.
	// input.OrderAmountCents é ignorado quando OrderID está presente — o frontend
	// não pode definir o valor de cobrança de um pedido.
	if input.OrderID != nil && payment.BundledOrderID == nil {
		order, err := tx.GetOrderForUpdate(ctx, input.BarbershopID, *input.OrderID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load order for payment: %w", err)
		}
		if order == nil {
			return nil, nil, apperr.ErrBusiness("order_not_found")
		}
		// Só pedidos pendentes podem ser vinculados a um pagamento.
		// Pedidos já pagos ou cancelados não devem gerar nova cobrança.
		if order.Status != models.OrderStatusPending {
			return nil, nil, apperr.ErrBusiness("order_not_linkable")
		}
		if order.TotalAmount <= 0 {
			return nil, nil, apperr.ErrBusiness("invalid_order_amount")
		}

		// Validação parcial de ownership: se o agendamento e o pedido tiverem
		// ClientID preenchidos, eles devem ser do mesmo cliente.
		// Se qualquer um for nil (encaixe manual, pedido anônimo), a checagem é ignorada
		// para evitar falso positivo — risco residual documentado abaixo.
		if payment.AppointmentID != nil && order.ClientID != nil {
			ap, err := tx.GetAppointmentForUpdate(ctx, input.BarbershopID, *payment.AppointmentID)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to load appointment for order validation: %w", err)
			}
			if ap != nil && ap.ClientID != nil && *ap.ClientID != *order.ClientID {
				return nil, nil, apperr.ErrBusiness("order_not_linkable")
			}
		}
		payment.Amount += order.TotalAmount
		payment.BundledOrderID = input.OrderID
		if err := tx.UpdatePaymentTx(ctx, input.BarbershopID, payment); err != nil {
			return nil, nil, fmt.Errorf("failed to update payment with order amount: %w", err)
		}
	}

	if payment.Amount < 100 {
		return nil, nil, apperr.ErrBusiness("invalid_amount")
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

	// Notification URL: usa o path correto para o provider em uso.
	// Gateways que implementam webhookPather declaram seu próprio path.
	// O fallback é o endpoint legado do Mercado Pago.
	// URL omitida quando o backend roda em localhost (provider rejeita URL não pública).
	notificationURL := ""
	if !strings.Contains(uc.backURL, "localhost") && !strings.Contains(uc.backURL, "127.0.0.1") {
		type webhookPather interface{ WebhookPath() string }
		webhookPath := "/api/webhooks/mp"
		if wp, ok := gateway.(webhookPather); ok {
			webhookPath = wp.WebhookPath()
		}
		notificationURL = uc.backURL + webhookPath
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
	// 6) Persistir TxID, QR code, provider e provider_payment_id
	// ==================================================
	// ProviderPaymentID tem precedência quando preenchido (PagBank e outros providers).
	// Fallback para MPPaymentID (Mercado Pago legado).
	var txid string
	if result.ProviderPaymentID != "" {
		txid = result.ProviderPaymentID
	} else {
		txid = mpPayPrefix + strconv.FormatInt(result.MPPaymentID, 10)
	}
	payment.TxID = &txid
	if result.QRCode != "" {
		payment.QRCode = &result.QRCode
	}

	// Grava o provider e o ID externo puro no payment para identificação futura.
	// provider_payment_id não carrega o prefixo interno "mp_pay:" — apenas o ID bruto.
	// Esses campos são lidos pelo polling de status (CheckPaymentStatus) para garantir
	// que o mesmo provider que criou o payment seja consultado, independentemente de
	// qual provider está atualmente ativo na barbearia.
	type providerNamer interface{ ProviderName() string }
	if pn, ok := gateway.(providerNamer); ok {
		name := pn.ProviderName()
		payment.Provider = &name
	}
	rawProviderID := strings.TrimPrefix(txid, mpPayPrefix)
	payment.ProviderPaymentID = &rawProviderID

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
		sendAppointmentConfirmedNotification(ctx, uc.db, uc.apptNotifier, uc.ticketRepo, uc.appURL, *payment.AppointmentID)
	}

	return payment, result, nil
}
