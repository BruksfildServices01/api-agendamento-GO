package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	appointmentDomain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type CreatePaymentForAppointment struct {
	paymentRepo     domain.Repository
	appointmentRepo appointmentDomain.Repository
	audit           *audit.Dispatcher
}

func NewCreatePaymentForAppointment(
	paymentRepo domain.Repository,
	appointmentRepo appointmentDomain.Repository,
	audit *audit.Dispatcher,
) *CreatePaymentForAppointment {
	return &CreatePaymentForAppointment{
		paymentRepo:     paymentRepo,
		appointmentRepo: appointmentRepo,
		audit:           audit,
	}
}

func (uc *CreatePaymentForAppointment) Execute(
	ctx context.Context,
	appointment *models.Appointment,
) (*models.Payment, error) {

	// --------------------------------------------------
	// 1️⃣ Validação básica
	// --------------------------------------------------
	if appointment == nil || appointment.ID == 0 {
		return nil, domain.ErrInvalidAppointment()
	}

	// --------------------------------------------------
	// 2️⃣ Idempotência por appointment
	// --------------------------------------------------
	existing, err := uc.paymentRepo.GetByAppointmentID(ctx, appointment.ID)
	if err == nil && existing != nil {
		return existing, nil
	}

	// --------------------------------------------------
	// 3️⃣ Busca explícita do produto (fonte da verdade)
	// --------------------------------------------------
	product, err := uc.appointmentRepo.GetProduct(
		ctx,
		appointment.BarbershopID,
		appointment.BarberProductID,
	)
	if err != nil || product == nil {
		return nil, domain.ErrInvalidAmount()
	}

	amount := product.Price
	if amount <= 0 {
		return nil, domain.ErrInvalidAmount()
	}

	// --------------------------------------------------
	// 4️⃣ Geração do TXID (Sprint G – determinístico)
	// --------------------------------------------------
	txid := fmt.Sprintf(
		"bs_%d_ap_%d",
		appointment.BarbershopID,
		appointment.ID,
	)

	now := time.Now()

	// --------------------------------------------------
	// 5️⃣ Criação do payment (já com TXID)
	// --------------------------------------------------
	payment := &models.Payment{
		BarbershopID:  appointment.BarbershopID,
		AppointmentID: appointment.ID,
		TxID:          &txid,
		Amount:        amount,
		Status:        string(domain.StatusPending),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := uc.paymentRepo.Create(ctx, payment); err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 6️⃣ Auditoria
	// --------------------------------------------------
	uc.audit.Dispatch(audit.Event{
		BarbershopID: appointment.BarbershopID,
		Action:       "payment_created",
		Entity:       "payment",
		EntityID:     &payment.ID,
		Metadata: map[string]any{
			"appointment_id": appointment.ID,
			"txid":           txid,
			"amount":         amount,
		},
	})

	return payment, nil
}
