package jobs

import (
	"context"
	"log"
	"time"

	domainAppointment "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

// orphanTTL define quanto tempo um appointment pode ficar awaiting_payment
// sem nenhum registro de payment antes de ser cancelado automaticamente.
const orphanTTL = 3 * time.Minute

type ExpirePaymentsJob struct {
	useCase    *ucPayment.ExpirePayments
	shopLister domainAppointment.BarbershopLister
	jobRepo    domainAppointment.JobRepository
}

func NewExpirePaymentsJob(
	useCase *ucPayment.ExpirePayments,
	shopLister domainAppointment.BarbershopLister,
	jobRepo domainAppointment.JobRepository,
) *ExpirePaymentsJob {
	return &ExpirePaymentsJob{
		useCase:    useCase,
		shopLister: shopLister,
		jobRepo:    jobRepo,
	}
}

func (j *ExpirePaymentsJob) Run(ctx context.Context) {
	now := time.Now().UTC()

	log.Printf("[ExpirePaymentsJob] started at=%s\n", now.Format(time.RFC3339))

	shops, err := j.shopLister.ListBarbershops(ctx)
	if err != nil {
		log.Printf("[ExpirePaymentsJob] failed listing barbershops error=%v\n", err)
		return
	}

	olderThan := now.Add(-orphanTTL)

	for _, shop := range shops {
		barbershopID := shop.ID

		if err := j.useCase.Execute(ctx, now, barbershopID); err != nil {
			log.Printf("[ExpirePaymentsJob] failed shop=%d error=%v\n", barbershopID, err)
			continue
		}

		// Cancela appointments awaiting_payment sem payment associado (clientes que abandonaram).
		if n, err := j.jobRepo.CancelOrphanAwaitingPayments(ctx, barbershopID, olderThan); err != nil {
			log.Printf("[ExpirePaymentsJob] orphan cleanup failed shop=%d error=%v\n", barbershopID, err)
		} else if n > 0 {
			log.Printf("[ExpirePaymentsJob] cancelled %d orphan appointments shop=%d\n", n, barbershopID)
		}
	}

	log.Printf("[ExpirePaymentsJob] finished at=%s\n", time.Now().UTC().Format(time.RFC3339))
}
