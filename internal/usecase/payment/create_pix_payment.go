package payment

import (
	"context"
	"fmt"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/idempotency"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type CreatePixPayment struct {
	repo  domain.Repository
	pix   domain.PixGateway
	audit *audit.Dispatcher
	idem  idempotency.Store
}

func NewCreatePixPayment(
	repo domain.Repository,
	pix domain.PixGateway,
	audit *audit.Dispatcher,
	idem idempotency.Store,
) *CreatePixPayment {
	return &CreatePixPayment{
		repo:  repo,
		pix:   pix,
		audit: audit,
		idem:  idem,
	}
}

func (uc *CreatePixPayment) Execute(
	ctx context.Context,
	barbershopID uint,
	appointmentID uint,
) (*models.Payment, *domain.PixCharge, error) {

	// ==================================================
	// 1) BEGIN TX (DB é a trava cross-instância)
	// ==================================================
	tx, err := uc.repo.BeginTx(ctx, barbershopID)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	// ==================================================
	// 2) Lock do payment por appointment (FOR UPDATE)
	// ==================================================
	payment, err := tx.GetByAppointmentIDForUpdate(ctx, barbershopID, appointmentID)
	if err != nil {
		return nil, nil, err
	}
	if payment == nil {
		return nil, nil, httperr.ErrBusiness("payment_not_found")
	}

	// Se já tem TxID, é retry idempotente: só devolve o mesmo
	if payment.TxID != nil {
		out := &domain.PixCharge{TxID: *payment.TxID}
		if payment.ExpiresAt != nil {
			out.ExpiresAt = *payment.ExpiresAt
		}
		if payment.QRCode != nil {
			out.QRCode = *payment.QRCode
		}
		if err := tx.Commit(); err != nil {
			return nil, nil, err
		}
		return payment, out, nil
	}

	// Só gera PIX se estiver pending
	if domain.Status(payment.Status) != domain.StatusPending {
		return nil, nil, httperr.ErrBusiness("payment_not_pending")
	}

	amountCents := payment.Amount
	if amountCents <= 100 {
		return nil, nil, httperr.ErrBusiness("invalid_amount")
	}
	amountFloat := float64(amountCents) / 100

	// ==================================================
	// 3) Criar charge PIX (único — segunda request fica bloqueada no lock)
	// ==================================================
	charge, err := uc.pix.CreateCharge(
		amountFloat,
		fmt.Sprintf("Agendamento #%d", appointmentID),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("pix charge creation failed: %w", err)
	}

	// ==================================================
	// 4) Persistir detalhes (ainda dentro do lock)
	// ==================================================
	payment.TxID = &charge.TxID

	if !charge.ExpiresAt.IsZero() {
		payment.ExpiresAt = &charge.ExpiresAt
	}
	if charge.QRCode != "" {
		qr := charge.QRCode
		payment.QRCode = &qr
	}

	if err := tx.UpdatePaymentTx(ctx, barbershopID, payment); err != nil {
		return nil, nil, fmt.Errorf("failed updating payment: %w", err)
	}

	// ==================================================
	// 5) COMMIT libera a trava
	// ==================================================
	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit failed: %w", err)
	}

	// ==================================================
	// 6) Auditoria (fora da tx)
	// ==================================================
	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		Action:       "payment_pix_charge_created",
		Entity:       "payment",
		EntityID:     &payment.ID,
		Metadata: map[string]any{
			"txid": charge.TxID,
		},
	})

	return payment, charge, nil
}
