package subscription

import (
	"context"
	"fmt"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

type ConsumeCutStatus string

const (
	ConsumeCutStatusConsumed             ConsumeCutStatus = "consumed"
	ConsumeCutStatusNoActiveSubscription ConsumeCutStatus = "no_active_subscription"
	ConsumeCutStatusExpiredPeriod        ConsumeCutStatus = "expired_period"
	ConsumeCutStatusPlanNotFound         ConsumeCutStatus = "plan_not_found"
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

	// --------------------------------------------------
	// 1) Buscar assinatura ativa
	// --------------------------------------------------
	sub, err := uc.repo.GetActiveSubscription(ctx, barbershopID, clientID)
	if err != nil {
		return nil, err
	}

	if sub == nil {
		return &ConsumeCutResult{
			Status: ConsumeCutStatusNoActiveSubscription,
		}, nil
	}

	result := &ConsumeCutResult{
		PlanID: &sub.PlanID,
	}

	// --------------------------------------------------
	// 2) Verificar período
	// --------------------------------------------------
	now := time.Now().UTC()

	if now.After(sub.CurrentPeriodEnd) {
		result.Status = ConsumeCutStatusExpiredPeriod
		return result, nil
	}

	// --------------------------------------------------
	// 3) Buscar plano para saber limite
	// --------------------------------------------------
	plans, err := uc.repo.ListPlans(ctx, barbershopID)
	if err != nil {
		return nil, err
	}

	var plan *domain.Plan
	for i := range plans {
		if plans[i].ID == sub.PlanID {
			plan = &plans[i]
			break
		}
	}

	if plan == nil {
		result.Status = ConsumeCutStatusPlanNotFound
		return result, nil
	}

	// --------------------------------------------------
	// 4) Validar se serviço está permitido no plano
	// --------------------------------------------------
	allowedServices, err := uc.repo.ListAllowedServiceIDs(ctx, sub.PlanID)
	if err != nil {
		return nil, err
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

	// --------------------------------------------------
	// 5) Limite de cortes
	// --------------------------------------------------
	if sub.CutsUsedInPeriod >= plan.CutsIncluded {
		result.Status = ConsumeCutStatusLimitExceeded
		return result, nil
	}

	// --------------------------------------------------
	// 6) Incrementar
	// --------------------------------------------------
	if err := uc.repo.IncrementCutsUsed(ctx, barbershopID, clientID); err != nil {
		return nil, fmt.Errorf("increment subscription cuts used: %w", err)
	}

	result.Status = ConsumeCutStatusConsumed
	return result, nil
}
