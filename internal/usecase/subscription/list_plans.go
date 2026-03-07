package subscription

import (
	"context"

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

	return uc.repo.ListPlans(ctx, barbershopID)
}
