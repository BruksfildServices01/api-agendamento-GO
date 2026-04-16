package appointment

import (
	"context"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
	ucSubscription "github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

type MarkAppointmentNoShow struct {
	repo      domain.Repository
	audit     *audit.Dispatcher
	metrics   *ucMetrics.UpdateClientMetrics
	releaseUC *ucSubscription.ReleaseSubscriptionCut
}

func NewMarkAppointmentNoShow(
	repo domain.Repository,
	audit *audit.Dispatcher,
	metrics *ucMetrics.UpdateClientMetrics,
	releaseUC *ucSubscription.ReleaseSubscriptionCut,
) *MarkAppointmentNoShow {
	return &MarkAppointmentNoShow{
		repo:      repo,
		audit:     audit,
		metrics:   metrics,
		releaseUC: releaseUC,
	}
}

func (uc *MarkAppointmentNoShow) Execute(
	ctx context.Context,
	barbershopID uint,
	barberID uint,
	appointmentID uint,
) error {

	ap, err := uc.repo.GetAppointmentForBarber(
		ctx,
		barbershopID,
		appointmentID,
		barberID,
	)
	if err != nil || ap == nil {
		return httperr.ErrBusiness("appointment_not_found")
	}

	// Segurança extra multi-tenant
	if ap.BarbershopID == nil || *ap.BarbershopID != barbershopID {
		return httperr.ErrBusiness("invalid_barbershop")
	}

	now := time.Now().UTC()

	if err := domain.MarkNoShow(ap, now, "manual"); err != nil {
		return err
	}

	if err := uc.repo.UpdateAppointment(ctx, ap); err != nil {
		return err
	}

	// Liberar reserva de assinatura (best-effort)
	if ap.ReservedSubscriptionCut && ap.ClientID != nil && ap.BarbershopID != nil && uc.releaseUC != nil {
		_ = uc.releaseUC.Execute(ctx, *ap.BarbershopID, *ap.ClientID)
	}

	if ap.ClientID != nil {
		_ = uc.metrics.Execute(ctx, ucMetrics.UpdateClientMetricsInput{
			BarbershopID: barbershopID,
			ClientID:     *ap.ClientID,
			EventType:    ucMetrics.EventAppointmentNoShow,
			OccurredAt:   now,
		})
	}

	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		UserID:       &barberID,
		Action:       "appointment_no_show",
		Entity:       "appointment",
		EntityID:     &ap.ID,
	})

	return nil
}
