package subscription

import (
	"context"
	"strings"

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
	DurationDays      int
	CutsIncluded      int
	DiscountPercent   int
	ServiceIDs        []uint
	CategoryIDs       []uint
}

func (uc *CreatePlan) Execute(
	ctx context.Context,
	input CreatePlanInput,
) (*domain.Plan, error) {
	if input.BarbershopID == 0 {
		return nil, ErrInvalidBarbershop
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrInvalidName
	}

	if input.MonthlyPriceCents < 0 {
		return nil, ErrInvalidPrice
	}

	if input.DurationDays <= 0 {
		return nil, ErrInvalidPlanDuration
	}

	if input.CutsIncluded <= 0 {
		return nil, ErrInvalidCutsIncluded
	}

	if input.DiscountPercent < 0 || input.DiscountPercent > 100 {
		return nil, ErrInvalidDiscount
	}

	if len(input.ServiceIDs) == 0 && len(input.CategoryIDs) == 0 {
		return nil, ErrServiceIDsRequired
	}

	for _, serviceID := range input.ServiceIDs {
		if serviceID == 0 {
			return nil, ErrInvalidServiceID
		}
	}

	if len(input.ServiceIDs) > 0 {
		count, err := uc.repo.CountServicesByIDs(
			ctx,
			input.BarbershopID,
			input.ServiceIDs,
		)
		if err != nil {
			return nil, err
		}
		if count != int64(len(input.ServiceIDs)) {
			return nil, ErrInvalidServiceIDs
		}
	}

	if len(input.CategoryIDs) > 0 {
		for _, catID := range input.CategoryIDs {
			if catID == 0 {
				return nil, ErrInvalidServiceID
			}
		}
		count, err := uc.repo.CountCategoriesByIDs(ctx, input.BarbershopID, input.CategoryIDs)
		if err != nil {
			return nil, err
		}
		if count != int64(len(input.CategoryIDs)) {
			return nil, ErrInvalidServiceIDs
		}
	}

	plan := &domain.Plan{
		BarbershopID:      input.BarbershopID,
		Name:              name,
		MonthlyPriceCents: input.MonthlyPriceCents,
		DurationDays:      input.DurationDays,
		CutsIncluded:      input.CutsIncluded,
		DiscountPercent:   input.DiscountPercent,
		Active:            true,
	}

	if err := uc.repo.CreatePlan(ctx, plan, input.ServiceIDs, input.CategoryIDs); err != nil {
		return nil, err
	}
	return plan, nil
}
