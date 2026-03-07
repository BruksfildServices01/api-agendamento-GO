package subscription

import (
	"context"
	"fmt"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

type ActivateSubscription struct {
	repo domain.Repository
}

func NewActivateSubscription(repo domain.Repository) *ActivateSubscription {
	return &ActivateSubscription{repo: repo}
}

type ActivateSubscriptionInput struct {
	BarbershopID uint
	ClientID     uint
	PlanID       uint
}

func (uc *ActivateSubscription) Execute(
	ctx context.Context,
	input ActivateSubscriptionInput,
) error {

	if input.ClientID == 0 || input.PlanID == 0 {
		return fmt.Errorf("invalid_input")
	}

	sub := &domain.Subscription{
		BarbershopID:       input.BarbershopID,
		ClientID:           input.ClientID,
		PlanID:             input.PlanID,
		Status:             domain.StatusActive,
		CurrentPeriodStart: time.Now().UTC(),
		CurrentPeriodEnd:   time.Now().UTC().AddDate(0, 1, 0),
		CutsUsedInPeriod:   0,
	}

	return uc.repo.ActivateSubscription(ctx, sub)
}
