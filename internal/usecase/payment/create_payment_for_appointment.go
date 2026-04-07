package payment

import (
	"context"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	appointmentDomain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	paymentconfig "github.com/BruksfildServices01/barber-scheduler/internal/domain/paymentconfig"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type CreatePaymentForAppointment struct {
	paymentRepo     domain.Repository
	appointmentRepo appointmentDomain.Repository
	paymentCfgRepo  paymentconfig.Repository
	audit           *audit.Dispatcher
}

func NewCreatePaymentForAppointment(
	paymentRepo domain.Repository,
	appointmentRepo appointmentDomain.Repository,
	paymentCfgRepo paymentconfig.Repository,
	audit *audit.Dispatcher,
) *CreatePaymentForAppointment {
	return &CreatePaymentForAppointment{
		paymentRepo:     paymentRepo,
		appointmentRepo: appointmentRepo,
		paymentCfgRepo:  paymentCfgRepo,
		audit:           audit,
	}
}

func (uc *CreatePaymentForAppointment) Execute(
	ctx context.Context,
	appointment *models.Appointment,
) (*models.Payment, error) {

	// --------------------------------------------------
	// 1) Validação básica
	// --------------------------------------------------
	if appointment == nil || appointment.ID == 0 {
		return nil, domain.ErrInvalidAppointment()
	}
	if appointment.BarbershopID == nil || appointment.BarberProductID == nil {
		return nil, domain.ErrInvalidAppointment()
	}

	barbershopID := *appointment.BarbershopID
	productID := *appointment.BarberProductID

	// --------------------------------------------------
	// 2) Carrega config (para expires_at)
	// --------------------------------------------------
	cfg, err := uc.paymentCfgRepo.GetByBarbershopID(ctx, barbershopID)
	if err != nil {
		return nil, err
	}

	expMinutes := cfg.PixExpirationMinutes
	if expMinutes <= 0 {
		expMinutes = 4
	}

	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(expMinutes) * time.Minute)

	// --------------------------------------------------
	// 3) Idempotência por appointment
	// --------------------------------------------------
	existing, err := uc.paymentRepo.GetByAppointmentID(ctx, barbershopID, appointment.ID)
	if err == nil && existing != nil {

		// legacy fix: se existir e não tiver expires_at (e ainda estiver pending), setamos
		if existing.Status == models.PaymentStatus(domain.StatusPending) && existing.ExpiresAt == nil {
			existing.ExpiresAt = &expiresAt
			_ = uc.paymentRepo.Update(ctx, existing) // best-effort
		}

		return existing, nil
	}

	// --------------------------------------------------
	// 4) Produto (fonte da verdade do amount)
	// --------------------------------------------------
	product, err := uc.appointmentRepo.GetProduct(ctx, barbershopID, productID)
	if err != nil || product == nil {
		return nil, domain.ErrInvalidAmount()
	}

	amountCents := product.Price
	if amountCents < 100 {
		return nil, domain.ErrInvalidAmount()
	}

	// --------------------------------------------------
	// 5) Criar payment
	//   - TxID: NÃO aqui (será definido ao criar charge PIX)
	//   - ExpiresAt: definido aqui (job de expiração depende disso)
	// --------------------------------------------------
	payment := &models.Payment{
		BarbershopID:  barbershopID,
		AppointmentID: &appointment.ID,
		Amount:        amountCents,
		Status:        models.PaymentStatus(domain.StatusPending),
		ExpiresAt:     &expiresAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := uc.paymentRepo.Create(ctx, payment); err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 6) Auditoria
	// --------------------------------------------------
	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		Action:       "payment_created",
		Entity:       "payment",
		EntityID:     &payment.ID,
		Metadata: map[string]any{
			"appointment_id": appointment.ID,
			"amount_cents":   amountCents,
			"expires_at":     expiresAt.Format(time.RFC3339),
		},
	})

	return payment, nil
}
