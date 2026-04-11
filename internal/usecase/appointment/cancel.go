package appointment

import (
	"context"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"

	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
)

type CancelAppointment struct {
	repo    domain.Repository
	audit   *audit.Dispatcher
	metrics *ucMetrics.UpdateClientMetrics
}

func NewCancelAppointment(
	repo domain.Repository,
	audit *audit.Dispatcher,
	metrics *ucMetrics.UpdateClientMetrics,
) *CancelAppointment {
	return &CancelAppointment{
		repo:    repo,
		audit:   audit,
		metrics: metrics,
	}
}

func (uc *CancelAppointment) Execute(
	ctx context.Context,
	barbershopID uint,
	barberID uint,
	appointmentID uint,
) (*models.Appointment, error) {

	// =========================================
	// 1️⃣ Appointment
	// =========================================
	// GetAppointmentForBarber already filters by barbershop_id, so a separate
	// GetBarbershopByID existence check is redundant — if the appointment is not
	// found the barbershop_id mismatch is caught below via *ap.BarbershopID.
	ap, err := uc.repo.GetAppointmentForBarber(
		ctx,
		barbershopID,
		appointmentID,
		barberID,
	)
	if err != nil || ap == nil {
		return nil, httperr.ErrBusiness("appointment_not_found")
	}

	if ap.BarbershopID == nil || *ap.BarbershopID != barbershopID {
		return nil, httperr.ErrBusiness("invalid_barbershop")
	}

	// =========================================
	// 2️⃣ Regra de domínio (✅ UTC para persistência)
	// =========================================
	now := time.Now().UTC()

	if err := domain.Cancel(ap, now); err != nil {
		return nil, err
	}

	// =========================================
	// 3️⃣ Persistência
	// =========================================
	if err := uc.repo.UpdateAppointment(ctx, ap); err != nil {
		return nil, err
	}

	// =========================================
	// 4️⃣ Auditoria
	// =========================================
	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		UserID:       &barberID,
		Action:       "appointment_canceled",
		Entity:       "appointment",
		EntityID:     &ap.ID,
	})

	// =========================================
	// 5️⃣ Métricas (best effort, ✅ UTC)
	// =========================================
	if ap.ClientID != nil {
		_ = uc.metrics.Execute(ctx, ucMetrics.UpdateClientMetricsInput{
			BarbershopID: barbershopID,
			ClientID:     *ap.ClientID,
			EventType:    ucMetrics.EventAppointmentCanceled,
			OccurredAt:   now,
		})
	}

	return ap, nil
}
