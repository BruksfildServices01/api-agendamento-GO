package jobs

import (
	"context"
	"log"
	"time"

	ucSubscription "github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

type ExpireSubscriptionsJob struct {
	useCase *ucSubscription.ExpireSubscriptions
}

func NewExpireSubscriptionsJob(useCase *ucSubscription.ExpireSubscriptions) *ExpireSubscriptionsJob {
	return &ExpireSubscriptionsJob{useCase: useCase}
}

func (j *ExpireSubscriptionsJob) Run(ctx context.Context) {
	now := time.Now().UTC()
	log.Printf("[ExpireSubscriptionsJob] started at=%s\n", now.Format(time.RFC3339))

	n, err := j.useCase.Execute(ctx)
	if err != nil {
		log.Printf("[ExpireSubscriptionsJob] error=%v\n", err)
		return
	}

	if n > 0 {
		log.Printf("[ExpireSubscriptionsJob] expired %d subscription(s)\n", n)
	}

	log.Printf("[ExpireSubscriptionsJob] finished at=%s\n", time.Now().UTC().Format(time.RFC3339))
}
