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
	if barbershopID == 0 || clientID == 0 {
		return nil, nil
	}

	sub, err := uc.repo.GetActiveSubscription(ctx, barbershopID, clientID)
	if err != nil {
		return nil, err
	}

	if sub == nil {
		return nil, nil
	}

	if sub.PlanID == 0 {
		sub.Plan = nil
		return sub, nil
	}

	plan, err := uc.repo.GetPlanByID(ctx, barbershopID, sub.PlanID)
	if err != nil {
		return nil, err
	}

	if plan == nil {
		sub.Plan = nil
		return sub, nil
	}

	serviceIDs, err := uc.repo.ListAllowedServiceIDs(ctx, plan.ID)
	if err != nil {
		return nil, err
	}

	plan.ServiceIDs = serviceIDs
	sub.Plan = plan

	return sub, nil
}
