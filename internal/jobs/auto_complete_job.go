package jobs

import (
	"context"
	"log"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domainAppointment "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
)

// AutoCompleteJob conclui automaticamente agendamentos que não foram
// finalizados pelo barbeiro, 1 hora após o end_time previsto.
type AutoCompleteJob struct {
	repo       domainAppointment.JobRepository
	audit      *audit.Dispatcher
	shopLister domainAppointment.BarbershopLister
}

func NewAutoCompleteJob(
	repo domainAppointment.JobRepository,
	auditDispatcher *audit.Dispatcher,
	shopLister domainAppointment.BarbershopLister,
) *AutoCompleteJob {
	return &AutoCompleteJob{
		repo:       repo,
		audit:      auditDispatcher,
		shopLister: shopLister,
	}
}

func (j *AutoCompleteJob) Run(ctx context.Context) error {
	shops, err := j.shopLister.ListBarbershops(ctx)
	if err != nil {
		return err
	}

	// Conclui agendamentos cujo end_time passou há mais de 1 hora
	const autoCompleteAfter = 1 * time.Hour

	for _, shop := range shops {
		cutoff := time.Now().UTC().Add(-autoCompleteAfter)

		count, err := j.repo.AutoCompleteAppointments(ctx, shop.ID, cutoff)
		if err != nil {
			log.Printf("[AutoCompleteJob] barbershop=%d error=%v", shop.ID, err)
			continue
		}

		if count > 0 {
			log.Printf("[AutoCompleteJob] barbershop=%d completed=%d", shop.ID, count)
			j.audit.Dispatch(audit.Event{
				BarbershopID: shop.ID,
				Action:       "appointments_auto_completed",
				Entity:       "appointment",
				Metadata: map[string]any{
					"count":  count,
					"cutoff": cutoff,
				},
			})
		}
	}

	return nil
}
