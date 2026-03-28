package subscription

import (
	"context"
	"errors"
	"fmt"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

var ErrConsumeCutInfra = errors.New("consume_cut_infra_error")

type ConsumeCutStatus string

const (
	ConsumeCutStatusConsumed             ConsumeCutStatus = "consumed"
	ConsumeCutStatusNoActiveSubscription ConsumeCutStatus = "no_active_subscription"
	ConsumeCutStatusExpiredPeriod        ConsumeCutStatus = "expired_period"
	ConsumeCutStatusPlanNotFound         ConsumeCutStatus = "plan_not_found"
	ConsumeCutStatusPlanInactive         ConsumeCutStatus = "plan_inactive"
	ConsumeCutStatusServiceNotAllowed    ConsumeCutStatus = "service_not_allowed"
	ConsumeCutStatusLimitExceeded        ConsumeCutStatus = "limit_exceeded"
)

type ConsumeCutResult struct {
	Status ConsumeCutStatus
	PlanID *uint
}

type ConsumeCut struct {
	repo domain.Repository
}

func NewConsumeCut(repo domain.Repository) *ConsumeCut {
	return &ConsumeCut{
		repo: repo,
	}
}

func (uc *ConsumeCut) Execute(
	ctx context.Context,
	barbershopID uint,
	clientID uint,
	serviceID uint,
) (*ConsumeCutResult, error) {
	if barbershopID == 0 || clientID == 0 || serviceID == 0 {
		return &ConsumeCutResult{
			Status: ConsumeCutStatusNoActiveSubscription,
		}, nil
	}

	sub, err := uc.repo.GetActiveSubscription(ctx, barbershopID, clientID)
	if err != nil {
		return nil, fmt.Errorf("%w: get active subscription: %v", ErrConsumeCutInfra, err)
	}

	if sub == nil {
		return &ConsumeCutResult{
			Status: ConsumeCutStatusNoActiveSubscription,
		}, nil
	}

	result := &ConsumeCutResult{
		PlanID: &sub.PlanID,
	}

	now := time.Now().UTC()
	if now.Before(sub.CurrentPeriodStart) || !now.Before(sub.CurrentPeriodEnd) {
		result.Status = ConsumeCutStatusExpiredPeriod
		return result, nil
	}

	plan, err := uc.repo.GetPlanByID(ctx, barbershopID, sub.PlanID)
	if err != nil {
		return nil, fmt.Errorf("%w: get plan: %v", ErrConsumeCutInfra, err)
	}

	if plan == nil {
		result.Status = ConsumeCutStatusPlanNotFound
		return result, nil
	}

	if !plan.Active {
		result.Status = ConsumeCutStatusPlanInactive
		return result, nil
	}

	allowedServices, err := uc.repo.ListAllowedServiceIDs(ctx, sub.PlanID)
	if err != nil {
		return nil, fmt.Errorf("%w: list allowed services: %v", ErrConsumeCutInfra, err)
	}

	isAllowed := false
	for _, id := range allowedServices {
		if id == serviceID {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		result.Status = ConsumeCutStatusServiceNotAllowed
		return result, nil
	}

	if sub.CutsUsedInPeriod >= plan.CutsIncluded {
		result.Status = ConsumeCutStatusLimitExceeded
		return result, nil
	}

	if err := uc.repo.IncrementCutsUsed(ctx, barbershopID, clientID); err != nil {
		if err.Error() == "active_subscription_not_found" {
			result.Status = ConsumeCutStatusNoActiveSubscription
			return result, nil
		}

		return nil, fmt.Errorf("%w: increment cuts used: %v", ErrConsumeCutInfra, err)
	}

	result.Status = ConsumeCutStatusConsumed
	return result, nil
}
