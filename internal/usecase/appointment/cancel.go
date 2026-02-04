package appointment

import (
	"context"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domainAppointment "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
)

type CancelAppointment struct {
	repo        domainAppointment.Repository
	paymentRepo domainPayment.Repository
	audit       *audit.Dispatcher
}

func NewCancelAppointment(
	repo domainAppointment.Repository,
	paymentRepo domainPayment.Repository,
	audit *audit.Dispatcher,
) *CancelAppointment {
	return &CancelAppointment{
		repo:        repo,
		paymentRepo: paymentRepo,
		audit:       audit,
	}
}

func (uc *CancelAppointment) Execute(
	ctx context.Context,
	barbershopID uint,
	barberID uint,
	appointmentID uint,
) (*models.Appointment, error) {

	// --------------------------------------------------
	// 1️⃣ Barbearia
	// --------------------------------------------------
	shop, err := uc.repo.GetBarbershopByID(ctx, barbershopID)
	if err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 2️⃣ Appointment
	// --------------------------------------------------
	ap, err := uc.repo.GetAppointmentForBarber(ctx, appointmentID, barberID)
	if err != nil {
		return nil, httperr.ErrBusiness("appointment_not_found")
	}

	// --------------------------------------------------
	// 🔒 3️⃣ Enforcement: bloqueia cancelamento se pago
	// --------------------------------------------------
	payment, err := uc.paymentRepo.GetByAppointmentID(ctx, ap.ID)
	if err == nil && payment != nil {
		if payment.Status == string(domainPayment.StatusPaid) {
			return nil, httperr.ErrBusiness("appointment_already_paid")
		}
	}

	// --------------------------------------------------
	// 4️⃣ Cancela (domínio)
	// --------------------------------------------------
	now := timezone.NowIn(shop.Timezone)
	if err := domainAppointment.Cancel(ap, now); err != nil {
		return nil, err
	}

	if err := uc.repo.UpdateAppointment(ctx, ap); err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 5️⃣ Auditoria
	// --------------------------------------------------
	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		UserID:       &barberID,
		Action:       "appointment_cancelled",
		Entity:       "appointment",
		EntityID:     &ap.ID,
	})

	return ap, nil
}
