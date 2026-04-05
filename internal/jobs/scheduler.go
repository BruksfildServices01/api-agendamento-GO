package jobs

import (
	"context"
	"time"
)

type Scheduler struct {
	ctx context.Context
}

func NewScheduler(ctx context.Context) *Scheduler {
	return &Scheduler{ctx: ctx}
}

func (s *Scheduler) Every(
	interval time.Duration,
	job func(context.Context),
) {
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				job(s.ctx)

			case <-s.ctx.Done():
				return
			}
		}
	}()
}
