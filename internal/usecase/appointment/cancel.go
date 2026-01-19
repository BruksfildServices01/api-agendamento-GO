package appointment

import (
	"context"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	"github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
)

type CancelAppointment struct {
	repo  domain.Repository
	audit *audit.Dispatcher
}

func NewCancelAppointment(
	repo domain.Repository,
	audit *audit.Dispatcher,
) *CancelAppointment {
	return &CancelAppointment{
		repo:  repo,
		audit: audit,
	}
}

func (uc *CancelAppointment) Execute(
	ctx context.Context,
	barbershopID uint,
	barberID uint,
	appointmentID uint,
) (*models.Appointment, error) {

	shop, err := uc.repo.GetBarbershopByID(ctx, barbershopID)
	if err != nil {
		return nil, err
	}

	ap, err := uc.repo.GetAppointmentForBarber(ctx, appointmentID, barberID)
	if err != nil {
		return nil, httperr.ErrBusiness("appointment_not_found")
	}

	now := timezone.NowIn(shop.Timezone)
	if err := appointment.Cancel(ap, now); err != nil {
		return nil, err
	}

	if err := uc.repo.UpdateAppointment(ctx, ap); err != nil {
		return nil, err
	}

	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		UserID:       &barberID,
		Action:       "appointment_cancelled",
		Entity:       "appointment",
		EntityID:     &ap.ID,
	})

	return ap, nil
}
