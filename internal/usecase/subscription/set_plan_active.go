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

	// Bloqueia desativação quando há assinantes ativos.
	// consume_cut.go verifica plan.Active antes de consumir corte — desativar o plano
	// faria o fechamento de atendimentos cobertos cair em cobrança normal indevidamente.
	// Enquanto não existir snapshot de plano por assinatura, o plano deve permanecer
	// ativo até que todos os assinantes encerrem o ciclo atual.
	if !active {
		count, err := uc.repo.CountActiveSubscribersByPlan(ctx, planID)
		if err != nil {
			return err
		}
		if count > 0 {
			return ErrPlanHasActiveSubscriptions
		}
	}

	if err := uc.repo.SetPlanActive(ctx, barbershopID, planID, active); err != nil {
		if err.Error() == "plan_not_found" {
			return ErrSetPlanActiveNotFound
		}
		return err
	}

	return nil
}
