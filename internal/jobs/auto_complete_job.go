package jobs

import (
	"context"
	"log"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domainAppointment "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	ucAppointment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/appointment"
)

// AutoCompleteJob conclui automaticamente agendamentos que não foram
// finalizados pelo barbeiro, 1 hora após o end_time previsto.
// O fechamento gerado replica o comportamento de uma conclusão manual com
// defaults: serviço agendado, sem itens adicionais, nota automática e
// método de pagamento derivado do pagamento existente (pix como padrão).
type AutoCompleteJob struct {
	completeUC *ucAppointment.CompleteAppointment
	repo       domainAppointment.JobRepository
	audit      *audit.Dispatcher
	shopLister domainAppointment.BarbershopLister
}

func NewAutoCompleteJob(
	completeUC *ucAppointment.CompleteAppointment,
	repo domainAppointment.JobRepository,
	auditDispatcher *audit.Dispatcher,
	shopLister domainAppointment.BarbershopLister,
) *AutoCompleteJob {
	return &AutoCompleteJob{
		completeUC: completeUC,
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

	const autoCompleteAfter = 1 * time.Hour

	for _, shop := range shops {
		cutoff := time.Now().UTC().Add(-autoCompleteAfter)

		candidates, err := j.repo.ListAutoCompleteCandidates(ctx, shop.ID, cutoff)
		if err != nil {
			log.Printf("[AutoCompleteJob] barbershop=%d list_error=%v", shop.ID, err)
			continue
		}

		completed := 0
		for _, c := range candidates {
			_, _, _, err := j.completeUC.Execute(ctx, ucAppointment.CompleteAppointmentInput{
				BarbershopID:          shop.ID,
				BarberID:              c.BarberID,
				AppointmentID:         c.AppointmentID,
				PaymentMethod:         c.PaymentMethod,
				ConfirmNormalCharging: true,
				OperationalNote:       "Concluído automaticamente pelo sistema",
			})
			if err != nil {
				log.Printf("[AutoCompleteJob] barbershop=%d appointment=%d error=%v", shop.ID, c.AppointmentID, err)
				continue
			}
			completed++
		}

		if completed > 0 {
			log.Printf("[AutoCompleteJob] barbershop=%d completed=%d", shop.ID, completed)
			j.audit.Dispatch(audit.Event{
				BarbershopID: shop.ID,
				Action:       "appointments_auto_completed",
				Entity:       "appointment",
				Metadata: map[string]any{
					"count":  completed,
					"cutoff": cutoff,
				},
			})
		}
	}

	return nil
}
