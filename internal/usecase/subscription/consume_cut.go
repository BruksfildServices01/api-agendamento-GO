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

// Execute consome um crédito de assinatura do cliente.
// repoOverride permite passar um repo vinculado a uma transação externa,
// garantindo que o consumo seja revertido se a transação falhar.
func (uc *ConsumeCut) Execute(
	ctx context.Context,
	barbershopID uint,
	clientID uint,
	serviceID uint,
	hadReservation bool,
	repoOverride ...domain.Repository,
) (*ConsumeCutResult, error) {
	repo := uc.repo
	if len(repoOverride) > 0 && repoOverride[0] != nil {
		repo = repoOverride[0]
	}

	if barbershopID == 0 || clientID == 0 || serviceID == 0 {
		return &ConsumeCutResult{
			Status: ConsumeCutStatusNoActiveSubscription,
		}, nil
	}

	sub, err := repo.GetActiveSubscription(ctx, barbershopID, clientID)
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

	plan, err := repo.GetPlanByID(ctx, barbershopID, sub.PlanID)
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

	// Quando hadReservation=true, o serviço foi validado contra o plano no momento
	// do booking. Se o dono alterou o plano depois, honramos a reserva original —
	// o cliente foi informado que o atendimento seria coberto.
	// Quando não há reserva (consumo direto), validamos o serviço atual do plano.
	if !hadReservation {
		allowedServices, err := repo.ListAllowedServiceIDs(ctx, sub.PlanID)
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
	}

	// When hadReservation is true we are converting an existing reservation into
	// a use (used+1, reserved-1 → net total unchanged). The relevant cap check is
	// whether used+1 would exceed the plan limit.
	//
	// When there is no reservation we are consuming a fresh cut; the full
	// committed count (used + reserved + 1) must fit within the plan.
	var postUsed int
	if hadReservation {
		postUsed = sub.CutsUsedInPeriod + 1
	} else {
		postUsed = sub.CutsUsedInPeriod + sub.CutsReservedInPeriod + 1
	}
	if postUsed > plan.CutsIncluded {
		result.Status = ConsumeCutStatusLimitExceeded
		return result, nil
	}

	var consumeErr error
	if hadReservation {
		consumeErr = repo.ConsumeReservedCut(ctx, barbershopID, clientID)
	} else {
		consumeErr = repo.IncrementCutsUsed(ctx, barbershopID, clientID)
	}

	if consumeErr != nil {
		switch {
		case errors.Is(consumeErr, domain.ErrActiveSubscriptionNotFound):
			if hadReservation {
				// A reserva ficou órfã: a assinatura foi cancelada ou expirou após o
				// booking. Tenta liberar o contador de reserva (best-effort — se a
				// assinatura não tem mais período ativo, ReleaseSubscriptionCut é no-op).
				_ = repo.ReleaseSubscriptionCut(ctx, barbershopID, clientID)
				// Retorna ExpiredPeriod para que complete.go exija confirmação de
				// cobrança normal, sinalizando ao barbeiro que o plano não está mais ativo.
				result.Status = ConsumeCutStatusExpiredPeriod
				return result, nil
			}
			result.Status = ConsumeCutStatusNoActiveSubscription
			return result, nil
		case errors.Is(consumeErr, domain.ErrCutsLimitExceeded):
			// Cap atingido no nível do banco (race condition prevenida atomicamente).
			result.Status = ConsumeCutStatusLimitExceeded
			return result, nil
		}
		return nil, fmt.Errorf("%w: consume cut: %v", ErrConsumeCutInfra, consumeErr)
	}

	result.Status = ConsumeCutStatusConsumed
	return result, nil
}
