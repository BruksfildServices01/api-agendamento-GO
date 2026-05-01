package appointment

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/apperr"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
	ucSubscription "github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

type MarkAppointmentNoShow struct {
	db               *gorm.DB
	repo             txableRepository
	subscriptionRepo txableSubscriptionRepo
	audit            *audit.Dispatcher
	metrics          *ucMetrics.UpdateClientMetrics
	releaseUC        *ucSubscription.ReleaseSubscriptionCut
}

func NewMarkAppointmentNoShow(
	db *gorm.DB,
	repo txableRepository,
	subscriptionRepo txableSubscriptionRepo,
	audit *audit.Dispatcher,
	metrics *ucMetrics.UpdateClientMetrics,
	releaseUC *ucSubscription.ReleaseSubscriptionCut,
) *MarkAppointmentNoShow {
	return &MarkAppointmentNoShow{
		db:               db,
		repo:             repo,
		subscriptionRepo: subscriptionRepo,
		audit:            audit,
		metrics:          metrics,
		releaseUC:        releaseUC,
	}
}

func (uc *MarkAppointmentNoShow) Execute(
	ctx context.Context,
	barbershopID uint,
	barberID uint,
	appointmentID uint,
) error {

	var noShowAt time.Time
	var clientID *uint
	var apID uint

	err := uc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := uc.repo.WithTx(tx)

		ap, err := txRepo.GetAppointmentForBarber(ctx, barbershopID, appointmentID, barberID)
		if err != nil || ap == nil {
			return apperr.ErrBusiness("appointment_not_found")
		}

		if ap.BarbershopID == nil || *ap.BarbershopID != barbershopID {
			return apperr.ErrBusiness("invalid_barbershop")
		}

		noShowAt = time.Now().UTC()
		clientID = ap.ClientID
		apID = ap.ID

		if err := domain.MarkNoShow(ap, noShowAt, "manual"); err != nil {
			return err
		}

		if err := txRepo.UpdateAppointment(ctx, ap); err != nil {
			return err
		}

		// Liberar reserva de assinatura dentro da mesma transação.
		if ap.ReservedSubscriptionCut && ap.ClientID != nil && ap.BarbershopID != nil && uc.releaseUC != nil {
			txSubRepo := uc.subscriptionRepo.WithTx(tx)
			if err := uc.releaseUC.Execute(ctx, *ap.BarbershopID, *ap.ClientID, txSubRepo); err != nil {
				log.Printf("[MarkNoShow] release subscription cut failed for client %d: %v", *ap.ClientID, err)
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	if clientID != nil {
		_ = uc.metrics.Execute(ctx, ucMetrics.UpdateClientMetricsInput{
			BarbershopID: barbershopID,
			ClientID:     *clientID,
			EventType:    ucMetrics.EventAppointmentNoShow,
			OccurredAt:   noShowAt,
		})
	}

	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		UserID:       &barberID,
		Action:       "appointment_no_show",
		Entity:       "appointment",
		EntityID:     &apID,
	})

	return nil
}
