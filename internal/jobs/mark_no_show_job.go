package jobs

import (
	"context"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domainAppointment "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
)

type MarkNoShowJob struct {
	repo       domainAppointment.JobRepository
	metrics    *ucMetrics.UpdateClientMetrics
	audit      *audit.Dispatcher
	shopLister domainAppointment.BarbershopLister
}

func NewMarkNoShowJob(
	repo domainAppointment.JobRepository,
	metrics *ucMetrics.UpdateClientMetrics,
	audit *audit.Dispatcher,
	shopLister domainAppointment.BarbershopLister,
) *MarkNoShowJob {
	return &MarkNoShowJob{
		repo:       repo,
		metrics:    metrics,
		audit:      audit,
		shopLister: shopLister,
	}
}

func (j *MarkNoShowJob) Run(ctx context.Context) error {

	shops, err := j.shopLister.ListBarbershops(ctx)
	if err != nil {
		return err
	}

	const noShowAfter = 5 * time.Hour

	for _, shop := range shops {
		nowUTC := time.Now().UTC()
		cutoff := nowUTC.Add(-noShowAfter)

		candidates, err := j.repo.ListNoShowCandidates(ctx, shop.ID, cutoff)
		if err != nil {
			continue
		}

		for _, ap := range candidates {
			ok, err := j.repo.MarkNoShowAuto(ctx, shop.ID, ap.ID, nowUTC)
			if err != nil || !ok {
				// !ok = alguém já mudou status (cancel/complete/etc)
				continue
			}

			if ap.ClientID != nil {
				_ = j.metrics.Execute(ctx, ucMetrics.UpdateClientMetricsInput{
					BarbershopID: shop.ID,
					ClientID:     *ap.ClientID,
					EventType:    ucMetrics.EventAppointmentNoShow,
					OccurredAt:   nowUTC,
				})
			}

			j.audit.Dispatch(audit.Event{
				BarbershopID: shop.ID,
				Action:       "appointment_no_show_auto",
				Entity:       "appointment",
				EntityID:     &ap.ID,
			})
		}
	}

	return nil
}
