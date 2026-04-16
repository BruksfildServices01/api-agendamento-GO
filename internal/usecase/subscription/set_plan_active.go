package subscription

import (
	"context"
	"errors"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

var ErrSetPlanActiveNotFound = errors.New("plan_not_found")

type SetPlanActive struct {
	repo domain.Repository
}

func NewSetPlanActive(repo domain.Repository) *SetPlanActive {
	return &SetPlanActive{repo: repo}
}

func (uc *SetPlanActive) Execute(ctx context.Context, barbershopID, planID uint, active bool) error {
	if barbershopID == 0 || planID == 0 {
		return ErrInvalidInput
	}

	if err := uc.repo.SetPlanActive(ctx, barbershopID, planID, active); err != nil {
		if err.Error() == "plan_not_found" {
			return ErrSetPlanActiveNotFound
		}
		return err
	}

	return nil
}
