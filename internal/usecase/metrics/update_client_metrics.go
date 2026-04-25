package metrics

import (
	"context"
	"time"

	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
	infraRepo "github.com/BruksfildServices01/barber-scheduler/internal/infra/repository"
)

type UpdateClientMetrics struct {
	repo *infraRepo.ClientMetricsGormRepository
	db   *gorm.DB
}

func NewUpdateClientMetrics(
	repo *infraRepo.ClientMetricsGormRepository,
	db *gorm.DB,
) *UpdateClientMetrics {
	return &UpdateClientMetrics{repo: repo, db: db}
}

type UpdateClientMetricsInput struct {
	BarbershopID uint
	ClientID     uint
	EventType    EventType
	OccurredAt   time.Time
	Amount       int64
}

type EventType string

const (
	EventAppointmentCreated         EventType = "appointment_created"
	EventAppointmentCompleted       EventType = "appointment_completed"
	EventAppointmentCanceled        EventType = "appointment_canceled"
	EventAppointmentNoShow          EventType = "appointment_no_show"
	EventAppointmentRescheduled     EventType = "appointment_rescheduled"
	EventAppointmentLateCanceled    EventType = "appointment_late_canceled"
	EventAppointmentLateRescheduled EventType = "appointment_late_rescheduled"
)

// Execute atualiza as métricas do cliente de forma atômica.
// O SELECT FOR UPDATE dentro da transação impede que dois eventos simultâneos
// para o mesmo cliente se sobreponham e percam um incremento.
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

	return uc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := uc.repo.WithTx(tx)

		m, err := txRepo.GetOrCreate(ctx, in.BarbershopID, in.ClientID)
		if err != nil {
			return err
		}

		switch in.EventType {
		case EventAppointmentCreated:
			m.OnAppointmentCreated(occurredAt)
		case EventAppointmentCompleted:
			m.OnAppointmentCompleted(occurredAt, in.Amount)
		case EventAppointmentCanceled:
			m.OnAppointmentCanceled(occurredAt)
		case EventAppointmentNoShow:
			m.OnAppointmentNoShow(occurredAt)
		case EventAppointmentRescheduled:
			m.OnAppointmentRescheduled(occurredAt, false)
		case EventAppointmentLateCanceled:
			m.OnAppointmentCanceled(occurredAt)
			m.OnLateCancellation(occurredAt)
		case EventAppointmentLateRescheduled:
			m.OnAppointmentRescheduled(occurredAt, true)
		default:
			return nil
		}

		return txRepo.Save(ctx, m)
	})
}

// ManualCategoryExpiresAt helper para o usecase de override de categoria.
func ManualCategoryExpiresAt(m *domain.ClientMetrics) *time.Time {
	return m.ManualCategoryExpiresAt
}
