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
		m.OnAppointmentCreated(in.OccurredAt)

	case EventAppointmentCompleted:
		m.OnAppointmentCompleted(
			in.OccurredAt,
			in.Amount,
		)

	case EventAppointmentCanceled:
		m.OnAppointmentCanceled(in.OccurredAt)

	case EventAppointmentNoShow:
		m.OnAppointmentNoShow(in.OccurredAt)

	default:
		return nil
	}

	return uc.repo.Save(ctx, m)
}
