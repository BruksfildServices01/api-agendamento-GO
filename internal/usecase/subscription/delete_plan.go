package subscription

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

type DeletePlan struct {
	repo domain.Repository
}

func NewDeletePlan(repo domain.Repository) *DeletePlan {
	return &DeletePlan{repo: repo}
}

func (uc *DeletePlan) Execute(ctx context.Context, barbershopID, planID uint) error {
	if barbershopID == 0 || planID == 0 {
		return ErrInvalidInput
	}

	plan, err := uc.repo.GetPlanByID(ctx, barbershopID, planID)
	if err != nil {
		return err
	}
	if plan == nil {
		return ErrPlanNotFound
	}

	count, err := uc.repo.CountActiveSubscriptionsByPlan(ctx, planID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrPlanHasActiveSubscriptions
	}

	return uc.repo.DeletePlan(ctx, barbershopID, planID)
}
