package subscription

import (
	"context"
	"fmt"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

type CreatePlan struct {
	repo domain.Repository
}

func NewCreatePlan(repo domain.Repository) *CreatePlan {
	return &CreatePlan{repo: repo}
}

type CreatePlanInput struct {
	BarbershopID      uint
	Name              string
	MonthlyPriceCents int64
	CutsIncluded      int
	DiscountPercent   int
}

func (uc *CreatePlan) Execute(
	ctx context.Context,
	input CreatePlanInput,
) error {

	if input.Name == "" {
		return fmt.Errorf("invalid_name")
	}

	if input.MonthlyPriceCents < 0 {
		return fmt.Errorf("invalid_price")
	}

	if input.DiscountPercent < 0 || input.DiscountPercent > 100 {
		return fmt.Errorf("invalid_discount")
	}

	plan := &domain.Plan{
		BarbershopID:      input.BarbershopID,
		Name:              input.Name,
		MonthlyPriceCents: input.MonthlyPriceCents,
		CutsIncluded:      input.CutsIncluded,
		DiscountPercent:   input.DiscountPercent,
		Active:            true,
	}

	return uc.repo.CreatePlan(ctx, plan)
}
