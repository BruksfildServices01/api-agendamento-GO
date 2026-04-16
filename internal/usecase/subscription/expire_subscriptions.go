package subscription

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

type ExpireSubscriptions struct {
	repo domain.Repository
}

func NewExpireSubscriptions(repo domain.Repository) *ExpireSubscriptions {
	return &ExpireSubscriptions{repo: repo}
}

func (uc *ExpireSubscriptions) Execute(ctx context.Context) (int64, error) {
	return uc.repo.ExpireSubscriptions(ctx)
}
