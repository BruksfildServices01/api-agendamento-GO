package subscription

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type PurchaseSubscriptionInput struct {
	BarbershopID    uint
	PlanID          uint
	ClientName      string
	ClientPhone     string
	PayerEmail      string
	PayerCPF        string
	PaymentMethodID string
	Token           string // cartão — vazio para PIX
	Installments    int
}

type PurchaseSubscriptionResult struct {
	SubscriptionID uint
	PaymentID      uint
	MPPaymentID    int64
	Status         string // "active" (card approved) ou "pending" (PIX)
	QRCode         string
	QRCodeBase64   string
	TicketURL      string
}

type PurchaseSubscription struct {
	subscriptionRepo domain.Repository
	paymentRepo      domainPayment.Repository
	gateway          domainPayment.TransparentGateway
	audit            *audit.Dispatcher
	db               *gorm.DB
	backendURL       string
}

func NewPurchaseSubscription(
	subscriptionRepo domain.Repository,
	paymentRepo domainPayment.Repository,
	gateway domainPayment.TransparentGateway,
	audit *audit.Dispatcher,
	db *gorm.DB,
	backendURL string,
) *PurchaseSubscription {
	return &PurchaseSubscription{
		subscriptionRepo: subscriptionRepo,
		paymentRepo:      paymentRepo,
		gateway:          gateway,
		audit:            audit,
		db:               db,
		backendURL:       backendURL,
	}
}

func (uc *PurchaseSubscription) Execute(
	ctx context.Context,
	in PurchaseSubscriptionInput,
	gatewayOverride ...domainPayment.TransparentGateway,
) (*PurchaseSubscriptionResult, error) {
	gw := uc.gateway
	if len(gatewayOverride) > 0 && gatewayOverride[0] != nil {
		gw = gatewayOverride[0]
	}

	// ── 1. Valida plano ──────────────────────────────────────────────────
	plan, err := uc.subscriptionRepo.GetPlanByID(ctx, in.BarbershopID, in.PlanID)
	if err != nil {
		return nil, err
	}
	if plan == nil || !plan.Active {
		return nil, httperr.ErrBusiness("plan_not_found")
	}

	// ── 2. Encontra ou cria cliente ───────────────────────────────────────
	client, err := uc.findOrCreateClient(ctx, in.BarbershopID, in.ClientName, in.ClientPhone)
	if err != nil {
		return nil, err
	}

	// ── 3. Cria subscription pending_payment ──────────────────────────────
	sub := &domain.Subscription{
		BarbershopID: in.BarbershopID,
		ClientID:     client.ID,
		PlanID:       plan.ID,
	}
	if err := uc.subscriptionRepo.CreatePendingSubscription(ctx, sub); err != nil {
		if httperr.IsBusiness(err, "") {
			return nil, err
		}
		if isAlreadyHasSubscriptionErr(err) {
			return nil, httperr.ErrBusiness("client_already_has_active_subscription")
		}
		return nil, err
	}

	// ── 4. Cria payment pendente vinculado à subscription ─────────────────
	now := time.Now().UTC()
	txID := fmt.Sprintf("sub_pending:%d:%d", sub.ID, now.UnixMilli())
	notifURL := strings.TrimRight(uc.backendURL, "/") + "/webhooks/mp"

	payment := &models.Payment{
		BarbershopID:   in.BarbershopID,
		SubscriptionID: &sub.ID,
		Amount:         plan.MonthlyPriceCents,
		Status:         models.PaymentStatus(domainPayment.StatusPending),
		TxID:           &txID,
	}
	if err := uc.paymentRepo.Create(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to create payment: %w", err)
	}

	// ── 5. Chama gateway ──────────────────────────────────────────────────
	result, err := gw.CreatePayment(domainPayment.TransparentPaymentInput{
		AmountCents:       plan.MonthlyPriceCents,
		Description:       "Assinatura " + plan.Name,
		ExternalReference: fmt.Sprintf("%d", payment.ID),
		NotificationURL:   notifURL,
		PayerEmail:        in.PayerEmail,
		PayerCPF:          in.PayerCPF,
		PaymentMethodID:   in.PaymentMethodID,
		Token:             in.Token,
		Installments:      in.Installments,
	})
	if err != nil {
		return nil, fmt.Errorf("gateway error: %w", err)
	}

	// ── 6. Resultado por status ───────────────────────────────────────────
	switch result.Status {
	case "approved":
		// Cartão aprovado imediatamente — ativa subscription
		paidAt := time.Now().UTC()
		payment.Status = models.PaymentStatus(domainPayment.StatusPaid)
		payment.PaidAt = &paidAt
		if err := uc.paymentRepo.Update(ctx, payment); err != nil {
			return nil, fmt.Errorf("failed to update payment: %w", err)
		}

		periodStart := paidAt
		periodEnd := periodStart.AddDate(0, 0, plan.DurationDays)
		if err := uc.subscriptionRepo.ActivateSubscriptionByID(ctx, sub.ID, periodStart, periodEnd); err != nil {
			return nil, fmt.Errorf("failed to activate subscription: %w", err)
		}

		uc.audit.Dispatch(audit.Event{
			BarbershopID: in.BarbershopID,
			Action:       "subscription_activated",
			Entity:       "subscription",
			EntityID:     &sub.ID,
			Metadata: map[string]any{
				"plan_id":    plan.ID,
				"client_id":  client.ID,
				"via":        "card_immediate",
			},
		})

		return &PurchaseSubscriptionResult{
			SubscriptionID: sub.ID,
			PaymentID:      payment.ID,
			MPPaymentID:    result.MPPaymentID,
			Status:         "active",
		}, nil

	case "rejected":
		// Pagamento recusado — marca payment como expirado, subscription permanece pending
		// (será limpa pelo job de expiração ou pode tentar novamente)
		payment.Status = models.PaymentStatus(domainPayment.StatusExpired)
		_ = uc.paymentRepo.Update(ctx, payment)
		return nil, httperr.ErrBusiness("payment_rejected")

	default:
		// PIX ou in_process — retorna dados para o cliente aguardar
		qrCode := result.QRCode
		payment.QRCode = &qrCode
		if result.MPPaymentID != 0 {
			payment.MPPaymentID = &result.MPPaymentID
		}
		_ = uc.paymentRepo.Update(ctx, payment)

		return &PurchaseSubscriptionResult{
			SubscriptionID: sub.ID,
			PaymentID:      payment.ID,
			MPPaymentID:    result.MPPaymentID,
			Status:         "pending",
			QRCode:         result.QRCode,
			QRCodeBase64:   result.QRCodeBase64,
			TicketURL:      result.TicketURL,
		}, nil
	}
}

// findOrCreateClient encontra o cliente por telefone na barbearia ou cria um novo.
func (uc *PurchaseSubscription) findOrCreateClient(
	ctx context.Context,
	barbershopID uint,
	name, phone string,
) (*models.Client, error) {
	phone = strings.Join(strings.Fields(phone), "")

	var client models.Client
	err := uc.db.WithContext(ctx).
		Where("barbershop_id = ? AND phone = ?", barbershopID, phone).
		First(&client).Error

	if err == nil {
		return &client, nil
	}
	if !isNotFound(err) {
		return nil, err
	}

	client = models.Client{
		BarbershopID: &barbershopID,
		Name:         name,
		Phone:        phone,
	}
	if err := uc.db.WithContext(ctx).Create(&client).Error; err != nil {
		return nil, err
	}
	return &client, nil
}

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func isAlreadyHasSubscriptionErr(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "client_already_has_active_subscription") ||
		strings.Contains(msg, "uq_subscriptions_one_pending_per_client_shop") ||
		strings.Contains(msg, "uq_subscriptions_one_active_per_client_shop")
}
