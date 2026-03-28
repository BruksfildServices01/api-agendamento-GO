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
}

func (uc *CreatePlan) Execute(
	ctx context.Context,
	input CreatePlanInput,
) error {
	if input.BarbershopID == 0 {
		return ErrInvalidBarbershop
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
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

	if len(input.ServiceIDs) == 0 {
		return ErrServiceIDsRequired
	}

	for _, serviceID := range input.ServiceIDs {
		if serviceID == 0 {
			return ErrInvalidServiceID
		}
	}

	count, err := uc.repo.CountServicesByIDs(
		ctx,
		input.BarbershopID,
		input.ServiceIDs,
	)
	if err != nil {
		return err
	}

	if count != int64(len(input.ServiceIDs)) {
		return ErrInvalidServiceIDs
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

	return uc.repo.CreatePlan(ctx, plan, input.ServiceIDs)
}
