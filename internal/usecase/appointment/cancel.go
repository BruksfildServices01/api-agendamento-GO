package appointment

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
	ucSubscription "github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

type CancelAppointment struct {
	db               *gorm.DB
	repo             txableRepository
	subscriptionRepo txableSubscriptionRepo
	audit            *audit.Dispatcher
	metrics          *ucMetrics.UpdateClientMetrics
	releaseUC        *ucSubscription.ReleaseSubscriptionCut
}

func NewCancelAppointment(
	db *gorm.DB,
	repo txableRepository,
	subscriptionRepo txableSubscriptionRepo,
	audit *audit.Dispatcher,
	metrics *ucMetrics.UpdateClientMetrics,
	releaseUC *ucSubscription.ReleaseSubscriptionCut,
) *CancelAppointment {
	return &CancelAppointment{
		db:               db,
		repo:             repo,
		subscriptionRepo: subscriptionRepo,
		audit:            audit,
		metrics:          metrics,
		releaseUC:        releaseUC,
	}
}

func (uc *CancelAppointment) Execute(
	ctx context.Context,
	barbershopID uint,
	barberID uint,
	appointmentID uint,
) (*models.Appointment, error) {

	var ap *models.Appointment
	var cancelledAt time.Time

	err := uc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := uc.repo.WithTx(tx)

		loaded, err := txRepo.GetAppointmentForBarber(ctx, barbershopID, appointmentID, barberID)
		if err != nil || loaded == nil {
			return httperr.ErrBusiness("appointment_not_found")
		}
		ap = loaded

		if ap.BarbershopID == nil || *ap.BarbershopID != barbershopID {
			return httperr.ErrBusiness("invalid_barbershop")
		}

		cancelledAt = time.Now().UTC()

		if err := domain.Cancel(ap, cancelledAt); err != nil {
			return err
		}

		if err := txRepo.UpdateAppointment(ctx, ap); err != nil {
			return err
		}

		// Liberar reserva de assinatura dentro da mesma transação.
		// Em caso de falha (ex.: período expirado), loga e segue — o cancelamento
		// não deve ser bloqueado por falha no release.
		if ap.ReservedSubscriptionCut && ap.ClientID != nil && ap.BarbershopID != nil && uc.releaseUC != nil {
			txSubRepo := uc.subscriptionRepo.WithTx(tx)
			if err := uc.releaseUC.Execute(ctx, *ap.BarbershopID, *ap.ClientID, txSubRepo); err != nil {
				log.Printf("[CancelAppointment] release subscription cut failed for client %d: %v", *ap.ClientID, err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		UserID:       &barberID,
		Action:       "appointment_canceled",
		Entity:       "appointment",
		EntityID:     &ap.ID,
	})

	if ap.ClientID != nil {
		_ = uc.metrics.Execute(ctx, ucMetrics.UpdateClientMetricsInput{
			BarbershopID: barbershopID,
			ClientID:     *ap.ClientID,
			EventType:    ucMetrics.EventAppointmentCanceled,
			OccurredAt:   cancelledAt,
		})
	}

	return ap, nil
}
