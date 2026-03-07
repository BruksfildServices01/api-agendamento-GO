package subscription

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

type CancelSubscription struct {
	repo domain.Repository
}

func NewCancelSubscription(repo domain.Repository) *CancelSubscription {
	return &CancelSubscription{repo: repo}
}

func (uc *CancelSubscription) Execute(
	ctx context.Context,
	barbershopID uint,
	clientID uint,
) error {

	return uc.repo.CancelSubscription(ctx, barbershopID, clientID)
}
