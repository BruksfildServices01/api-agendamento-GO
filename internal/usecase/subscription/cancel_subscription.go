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
	if barbershopID == 0 || clientID == 0 {
		return ErrInvalidInput
	}

	err := uc.repo.CancelSubscription(ctx, barbershopID, clientID)
	if err != nil {
		if err.Error() == "active_subscription_not_found" {
			return ErrActiveSubscriptionNotFound
		}
		return err
	}

	return nil
}
