package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domainOrder "github.com/BruksfildServices01/barber-scheduler/internal/domain/order"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type CreatePixPaymentForOrder struct {
	paymentRepo domainPayment.Repository
	pixGateway  domainPayment.PixGateway
	audit       *audit.Dispatcher
}

func NewCreatePixPaymentForOrder(
	paymentRepo domainPayment.Repository,
	pixGateway domainPayment.PixGateway,
	audit *audit.Dispatcher,
) *CreatePixPaymentForOrder {
	return &CreatePixPaymentForOrder{
		paymentRepo: paymentRepo,
		pixGateway:  pixGateway,
		audit:       audit,
	}
}

type CreatePixPaymentForOrderOutput struct {
	PaymentID uint      `json:"payment_id"`
	TxID      string    `json:"txid"`
	QRCode    string    `json:"qr_code"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (uc *CreatePixPaymentForOrder) Execute(
	ctx context.Context,
	order *domainOrder.Order,
) (*CreatePixPaymentForOrderOutput, error) {

	// 1) Validações mínimas
	if order == nil || order.ID == 0 {
		return nil, fmt.Errorf("invalid_order")
	}
	barbershopID := order.BarbershopID
	if barbershopID == 0 {
		return nil, fmt.Errorf("invalid_barbershop")
	}

	// 2) BEGIN TX
	tx, err := uc.paymentRepo.BeginTx(ctx, barbershopID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 3) Lock do ORDER (serializa concorrência cross-instância)
	lockedOrder, err := tx.GetOrderForUpdate(ctx, barbershopID, order.ID)
	if err != nil {
		return nil, err
	}
	if lockedOrder == nil {
		return nil, fmt.Errorf("order_not_found")
	}
	if lockedOrder.Status != models.OrderStatusPending {
		return nil, fmt.Errorf("order_not_pending")
	}

	amountCents := lockedOrder.TotalAmount
	if amountCents < 100 {
		return nil, fmt.Errorf("invalid_amount")
	}

	// 4) Reusar payment existente (idempotência por order)
	p, err := tx.GetByOrderID(ctx, barbershopID, lockedOrder.ID)
	if err != nil {
		return nil, err
	}

	// Se existir um payment pendente com TxID => retorna o mesmo (retry/duplo clique)
	if p != nil && domainPayment.Status(p.Status) == domainPayment.StatusPending && p.TxID != nil {
		out := buildOrderPixOutput(p)
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return out, nil
	}

	now := time.Now().UTC()

	// 5) Se não existe payment pendente “usável”, cria um novo (sem txid ainda)
	// Observação: mantém o produto simples e evita duplicar por concorrência (lock no order).
	// Se você quiser impedir histórico múltiplo, dá pra mudar para “reusar sempre o mesmo row”.
	if p == nil || domainPayment.Status(p.Status) != domainPayment.StatusPending {
		expiresAt := now.Add(15 * time.Minute)

		p = &models.Payment{
			BarbershopID: barbershopID,
			OrderID:      &lockedOrder.ID,
			Amount:       amountCents,
			Status:       models.PaymentStatus(domainPayment.StatusPending),
			ExpiresAt:    &expiresAt,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := tx.Create(ctx, p); err != nil {
			return nil, err
		}
	}

	// 6) Gerar charge PIX (gateway)
	charge, err := uc.pixGateway.CreateCharge(
		float64(p.Amount)/100.0,
		fmt.Sprintf("Pedido #%d", lockedOrder.ID),
	)
	if err != nil {
		return nil, err
	}

	// 7) Persistir txid/qr/expires no MESMO row (ainda sob lock do order)
	txid := charge.TxID
	p.TxID = &txid

	if !charge.ExpiresAt.IsZero() {
		p.ExpiresAt = &charge.ExpiresAt
	}

	if charge.QRCode != "" {
		qr := charge.QRCode
		p.QRCode = &qr
	}

	if err := tx.UpdatePaymentTx(ctx, barbershopID, p); err != nil {
		return nil, err
	}

	// 8) COMMIT
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// 9) Auditoria (best-effort)
	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		Action:       "pix_created_for_order",
		Entity:       "payment",
		EntityID:     &p.ID,
		Metadata: map[string]any{
			"order_id": lockedOrder.ID,
			"txid":     txid,
		},
	})

	return buildOrderPixOutput(p), nil
}

func buildOrderPixOutput(p *models.Payment) *CreatePixPaymentForOrderOutput {
	qr := ""
	if p.QRCode != nil {
		qr = *p.QRCode
	}

	var exp time.Time
	if p.ExpiresAt != nil {
		exp = *p.ExpiresAt
	}

	txid := ""
	if p.TxID != nil {
		txid = *p.TxID
	}

	return &CreatePixPaymentForOrderOutput{
		PaymentID: p.ID,
		TxID:      txid,
		QRCode:    qr,
		ExpiresAt: exp,
	}
}
