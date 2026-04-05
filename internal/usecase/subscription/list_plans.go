package subscription

import (
	"context"
	"fmt"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

type ListPlans struct {
	repo domain.Repository
}

func NewListPlans(repo domain.Repository) *ListPlans {
	return &ListPlans{repo: repo}
}

func (uc *ListPlans) Execute(
	ctx context.Context,
	barbershopID uint,
) ([]domain.Plan, error) {
	if barbershopID == 0 {
		return nil, fmt.Errorf("invalid_barbershop")
	}

	plans, err := uc.repo.ListPlans(ctx, barbershopID)
	if err != nil {
		return nil, err
	}

	for i := range plans {
		serviceIDs, err := uc.repo.ListAllowedServiceIDs(ctx, plans[i].ID)
		if err != nil {
			return nil, err
		}
		plans[i].ServiceIDs = serviceIDs

		count, err := uc.repo.CountActiveSubscribersByPlan(ctx, plans[i].ID)
		if err != nil {
			return nil, err
		}
		plans[i].ActiveSubscribers = int(count)
	}

	return plans, nil
}
