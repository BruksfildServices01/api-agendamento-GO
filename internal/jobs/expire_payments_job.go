package jobs

import (
	"context"
	"log"
	"time"

	domainAppointment "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

type ExpirePaymentsJob struct {
	useCase    *ucPayment.ExpirePayments
	shopLister domainAppointment.BarbershopLister
}

func NewExpirePaymentsJob(
	useCase *ucPayment.ExpirePayments,
	shopLister domainAppointment.BarbershopLister,
) *ExpirePaymentsJob {
	return &ExpirePaymentsJob{
		useCase:    useCase,
		shopLister: shopLister,
	}
}

func (j *ExpirePaymentsJob) Run(ctx context.Context) {
	now := time.Now().UTC()

	log.Printf("[ExpirePaymentsJob] started at=%s\n", now.Format(time.RFC3339))

	// 🔥 1️⃣ Buscar todos tenants
	shops, err := j.shopLister.ListBarbershops(ctx)
	if err != nil {
		log.Printf("[ExpirePaymentsJob] failed listing barbershops error=%v\n", err)
		return
	}

	// 🔥 2️⃣ Executar para cada tenant
	for _, shop := range shops {
		barbershopID := shop.ID
		if err := j.useCase.Execute(ctx, now, barbershopID); err != nil {
			log.Printf(
				"[ExpirePaymentsJob] failed shop=%d error=%v\n",
				barbershopID,
				err,
			)
			continue
		}
	}

	log.Printf("[ExpirePaymentsJob] finished at=%s\n", time.Now().UTC().Format(time.RFC3339))
}
