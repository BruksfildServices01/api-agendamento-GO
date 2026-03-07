package subscription

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

type GetActiveSubscription struct {
	repo domain.Repository
}

func NewGetActiveSubscription(repo domain.Repository) *GetActiveSubscription {
	return &GetActiveSubscription{repo: repo}
}

func (uc *GetActiveSubscription) Execute(
	ctx context.Context,
	barbershopID uint,
	clientID uint,
) (*domain.Subscription, error) {

	return uc.repo.GetActiveSubscription(ctx, barbershopID, clientID)
}
