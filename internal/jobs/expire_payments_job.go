package jobs

import (
	"context"
	"log"
	"time"

	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

type ExpirePaymentsJob struct {
	useCase *ucPayment.ExpirePayments
}

func NewExpirePaymentsJob(
	useCase *ucPayment.ExpirePayments,
) *ExpirePaymentsJob {
	return &ExpirePaymentsJob{
		useCase: useCase,
	}
}

func (j *ExpirePaymentsJob) Run(ctx context.Context) {
	now := time.Now().UTC()

	log.Println("[ExpirePaymentsJob] running at", now)

	if err := j.useCase.Execute(ctx, now); err != nil {
		log.Println("[ExpirePaymentsJob] error:", err)
	}
}
