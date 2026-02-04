package payment

import (
	"context"
	"fmt"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type CreatePixPayment struct {
	repo  domain.Repository
	pix   domain.PixGateway
	audit *audit.Dispatcher
}

func NewCreatePixPayment(
	repo domain.Repository,
	pix domain.PixGateway,
	audit *audit.Dispatcher,
) *CreatePixPayment {
	return &CreatePixPayment{
		repo:  repo,
		pix:   pix,
		audit: audit,
	}
}

func (uc *CreatePixPayment) Execute(
	ctx context.Context,
	barbershopID uint,
	appointmentID uint,
	amount float64,
) (*models.Payment, *domain.PixCharge, error) {

	// 1️⃣ Bloqueia duplicação
	existing, err := uc.repo.GetByAppointmentID(ctx, appointmentID)
	if err != nil {
		return nil, nil, err
	}
	if existing != nil {
		return nil, nil, httperr.ErrBusiness("payment_already_exists")
	}

	// 2️⃣ Cria cobrança PIX
	charge, err := uc.pix.CreateCharge(
		amount,
		fmt.Sprintf("Agendamento #%d", appointmentID),
	)
	if err != nil {
		return nil, nil, err
	}

	// 3️⃣ Cria payment JÁ com txid
	payment := &models.Payment{
		BarbershopID:  barbershopID,
		AppointmentID: appointmentID,
		TxID:          &charge.TxID,
		Amount:        amount,
		Status:        string(domain.StatusPending),
		ExpiresAt:     &charge.ExpiresAt,
	}

	if err := uc.repo.Create(ctx, payment); err != nil {
		return nil, nil, err
	}

	// 4️⃣ Auditoria
	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		Action:       "payment_created",
		Entity:       "payment",
		EntityID:     &payment.ID,
		Metadata: map[string]any{
			"txid": charge.TxID,
		},
	})

	return payment, charge, nil
}
