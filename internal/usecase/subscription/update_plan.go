package subscription

import (
	"context"
	"errors"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

var ErrPlanUpdateNotFound = errors.New("plan_not_found")

type UpdatePlanInput struct {
	BarbershopID      uint
	PlanID            uint
	Name              string
	MonthlyPriceCents int64
	DurationDays      int
	CutsIncluded      int
	DiscountPercent   int
	ServiceIDs        []uint
	CategoryIDs       []uint
}

type UpdatePlan struct {
	repo domain.Repository
}

func NewUpdatePlan(repo domain.Repository) *UpdatePlan {
	return &UpdatePlan{repo: repo}
}

func (uc *UpdatePlan) Execute(ctx context.Context, input UpdatePlanInput) error {
	if input.BarbershopID == 0 || input.PlanID == 0 {
		return ErrInvalidInput
	}
	if input.Name == "" {
		return ErrInvalidName
	}
	if input.MonthlyPriceCents < 0 {
		return ErrInvalidPrice
	}
	if input.DurationDays <= 0 {
		return ErrInvalidPlanDuration
	}
	if input.CutsIncluded < 0 {
		return ErrInvalidCutsIncluded
	}
	if input.DiscountPercent < 0 || input.DiscountPercent > 100 {
		return ErrInvalidDiscount
	}

	if len(input.ServiceIDs) == 0 && len(input.CategoryIDs) == 0 {
		return ErrServiceIDsRequired
	}

	if len(input.ServiceIDs) > 0 {
		count, err := uc.repo.CountServicesByBarbershop(ctx, input.BarbershopID, input.ServiceIDs)
		if err != nil {
			return err
		}
		if count != int64(len(input.ServiceIDs)) {
			return ErrInvalidServiceIDs
		}
	}

	if len(input.CategoryIDs) > 0 {
		count, err := uc.repo.CountCategoriesByIDs(ctx, input.BarbershopID, input.CategoryIDs)
		if err != nil {
			return err
		}
		if count != int64(len(input.CategoryIDs)) {
			return ErrInvalidServiceIDs
		}
	}

	plan := &domain.Plan{
		Name:              input.Name,
		MonthlyPriceCents: input.MonthlyPriceCents,
		DurationDays:      input.DurationDays,
		CutsIncluded:      input.CutsIncluded,
		DiscountPercent:   input.DiscountPercent,
	}

	if err := uc.repo.UpdatePlan(ctx, input.BarbershopID, input.PlanID, plan, input.ServiceIDs, input.CategoryIDs); err != nil {
		if err.Error() == "plan_not_found" {
			return ErrPlanUpdateNotFound
		}
		return err
	}

	return nil
}
