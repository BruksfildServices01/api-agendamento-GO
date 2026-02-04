package appointment

import (
	"context"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
)

type CompleteAppointment struct {
	repo        domain.Repository
	paymentRepo domainPayment.Repository
	audit       *audit.Dispatcher
}

func NewCompleteAppointment(
	repo domain.Repository,
	paymentRepo domainPayment.Repository,
	audit *audit.Dispatcher,
) *CompleteAppointment {
	return &CompleteAppointment{
		repo:        repo,
		paymentRepo: paymentRepo,
		audit:       audit,
	}
}

func (uc *CompleteAppointment) Execute(
	ctx context.Context,
	barbershopID uint,
	barberID uint,
	appointmentID uint,
) (*models.Appointment, error) {

	// --------------------------------------------------
	// 1️⃣ Barbearia (timezone correto)
	// --------------------------------------------------
	shop, err := uc.repo.GetBarbershopByID(ctx, barbershopID)
	if err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 2️⃣ Appointment
	// --------------------------------------------------
	ap, err := uc.repo.GetAppointmentForBarber(
		ctx,
		appointmentID,
		barberID,
	)
	if err != nil {
		return nil, httperr.ErrBusiness("appointment_not_found")
	}

	// --------------------------------------------------
	// 🔒 ENFORCEMENT — pagamento obrigatório
	// --------------------------------------------------
	if ap.Status == string(domain.StatusAwaitingPayment) {

		payment, err := uc.paymentRepo.GetByAppointmentID(
			ctx,
			ap.ID,
		)
		if err != nil {
			return nil, httperr.ErrBusiness("payment_required")
		}

		if payment.Status != string(domainPayment.StatusPaid) {
			return nil, httperr.ErrBusiness("appointment_payment_not_paid")
		}
	}

	// --------------------------------------------------
	// 3️⃣ Regra de domínio
	// --------------------------------------------------
	now := timezone.NowIn(shop.Timezone)
	if err := domain.Complete(ap, now); err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 4️⃣ Persistência
	// --------------------------------------------------
	if err := uc.repo.UpdateAppointment(ctx, ap); err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 5️⃣ Auditoria
	// --------------------------------------------------
	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		UserID:       &barberID,
		Action:       "appointment_completed",
		Entity:       "appointment",
		EntityID:     &ap.ID,
	})

	return ap, nil
}
