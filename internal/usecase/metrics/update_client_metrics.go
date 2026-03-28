package metrics

import (
	"context"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
)

type UpdateClientMetrics struct {
	repo domain.ClientMetricsRepository
}

func NewUpdateClientMetrics(
	repo domain.ClientMetricsRepository,
) *UpdateClientMetrics {
	return &UpdateClientMetrics{
		repo: repo,
	}
}

// --------------------------------
// INPUT
// --------------------------------

type UpdateClientMetricsInput struct {
	BarbershopID uint
	ClientID     uint
	EventType    EventType
	OccurredAt   time.Time
	Amount       int64
}

type EventType string

const (
	EventAppointmentCreated   EventType = "appointment_created"
	EventAppointmentCompleted EventType = "appointment_completed"
	EventAppointmentCanceled  EventType = "appointment_canceled"
	EventAppointmentNoShow    EventType = "appointment_no_show"
)

// --------------------------------
// EXECUTE
// --------------------------------

func (uc *UpdateClientMetrics) Execute(
	ctx context.Context,
	in UpdateClientMetricsInput,
) error {

	if in.BarbershopID == 0 || in.ClientID == 0 {
		return nil
	}

	occurredAt := in.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	m, err := uc.repo.GetOrCreate(
		ctx,
		in.BarbershopID,
		in.ClientID,
	)
	if err != nil {
		return err
	}

	switch in.EventType {

	case EventAppointmentCreated:
		m.OnAppointmentCreated(occurredAt)

	case EventAppointmentCompleted:
		m.OnAppointmentCompleted(
			occurredAt,
			in.Amount,
		)

	case EventAppointmentCanceled:
		m.OnAppointmentCanceled(occurredAt)

	case EventAppointmentNoShow:
		m.OnAppointmentNoShow(occurredAt)

	case EventType("appointment_rescheduled"):
		m.OnAppointmentRescheduled(occurredAt, false)

	case EventType("appointment_late_canceled"):
		m.OnAppointmentCanceled(occurredAt)
		m.OnLateCancellation(occurredAt)

	case EventType("appointment_late_rescheduled"):
		m.OnAppointmentRescheduled(occurredAt, true)

	default:
		return nil
	}

	return uc.repo.Save(ctx, m)
}
