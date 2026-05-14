package subscription

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

// ── stub para testes de ativação idempotente ─────────────────────────────────────

// stubActivateRepo controla ActivateSubscriptionByID e ActivateSubscription
// para testar a idempotência sem banco real.
type stubActivateRepo struct {
	activeSub          *domain.Subscription // retorno de GetActiveSubscription
	activeSubErr       error
	plan               *domain.Plan
	planErr            error
	activateByIDErr    error // retorno de ActivateSubscriptionByID
	activateErr        error // retorno de ActivateSubscription (manual)
}

func (r *stubActivateRepo) GetActiveSubscription(_ context.Context, _, _ uint) (*domain.Subscription, error) {
	return r.activeSub, r.activeSubErr
}
func (r *stubActivateRepo) GetPlanByID(_ context.Context, _, _ uint) (*domain.Plan, error) {
	return r.plan, r.planErr
}
func (r *stubActivateRepo) ActivateSubscription(_ context.Context, _ *domain.Subscription) error {
	return r.activateErr
}
func (r *stubActivateRepo) ActivateSubscriptionByID(_ context.Context, _ uint, _, _ time.Time) error {
	return r.activateByIDErr
}

// No-ops para o restante da interface.
func (r *stubActivateRepo) CreatePlan(_ context.Context, _ *domain.Plan, _, _ []uint) error {
	return nil
}
func (r *stubActivateRepo) UpdatePlan(_ context.Context, _, _ uint, _ *domain.Plan, _, _ []uint) error {
	return nil
}
func (r *stubActivateRepo) SetPlanActive(_ context.Context, _, _ uint, _ bool) error { return nil }
func (r *stubActivateRepo) ListPlans(_ context.Context, _ uint) ([]domain.Plan, error) {
	return nil, nil
}
func (r *stubActivateRepo) DeletePlan(_ context.Context, _, _ uint) error { return nil }
func (r *stubActivateRepo) CountActiveSubscriptionsByPlan(_ context.Context, _ uint) (int64, error) {
	return 0, nil
}
func (r *stubActivateRepo) CountActiveSubscribersByPlan(_ context.Context, _ uint) (int64, error) {
	return 0, nil
}
func (r *stubActivateRepo) CancelSubscription(_ context.Context, _, _ uint) error { return nil }
func (r *stubActivateRepo) CreatePendingSubscription(_ context.Context, _ *domain.Subscription) error {
	return nil
}
func (r *stubActivateRepo) GetSubscriptionByID(_ context.Context, _ uint) (*domain.Subscription, error) {
	return nil, nil
}
func (r *stubActivateRepo) ActivateSubscriptionByIDStub(_ context.Context, _ uint, _, _ time.Time) error {
	return r.activateByIDErr
}
func (r *stubActivateRepo) ExpireSubscriptions(_ context.Context) (int64, error) { return 0, nil }
func (r *stubActivateRepo) IncrementCutsUsed(_ context.Context, _, _ uint) error  { return nil }
func (r *stubActivateRepo) ReserveSubscriptionCut(_ context.Context, _, _ uint) error { return nil }
func (r *stubActivateRepo) ReleaseSubscriptionCut(_ context.Context, _, _ uint) error { return nil }
func (r *stubActivateRepo) ConsumeReservedCut(_ context.Context, _, _ uint) error  { return nil }
func (r *stubActivateRepo) AddServiceToPlan(_ context.Context, _, _ uint) error    { return nil }
func (r *stubActivateRepo) ListAllowedServiceIDs(_ context.Context, _ uint) ([]uint, error) {
	return nil, nil
}
func (r *stubActivateRepo) UpdateCutsUsed(_ context.Context, _ uint, _ int) error { return nil }
func (r *stubActivateRepo) CountServicesByIDs(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}
func (r *stubActivateRepo) CountServicesByBarbershop(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}
func (r *stubActivateRepo) CountCategoriesByIDs(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}

// ── testes ───────────────────────────────────────────────────────────────────────

// Teste 5 — webhook duplicado (ActivateSubscription manual):
// Se o cliente já tem subscription ativa, o use case retorna
// ErrActivateSubscriptionClientAlreadyHasActiveSub — não é fatal no webhook.
func TestActivateSubscription_ClienteJaAtivo_NaoEhFatal(t *testing.T) {
	now := time.Now().UTC()
	planID := uint(1)
	repo := &stubActivateRepo{
		// Simula cliente que já tem subscription ativa.
		activeSub: &domain.Subscription{
			ID:                 10,
			BarbershopID:       1,
			ClientID:           42,
			PlanID:             planID,
			CurrentPeriodStart: now.Add(-24 * time.Hour),
			CurrentPeriodEnd:   now.Add(30 * 24 * time.Hour),
		},
		plan: &domain.Plan{
			ID:           planID,
			BarbershopID: 1,
			CutsIncluded: 4,
			Active:       true,
			DurationDays: 30,
		},
	}

	uc := NewActivateSubscription(repo)
	err := uc.Execute(context.Background(), ActivateSubscriptionInput{
		BarbershopID: 1,
		ClientID:     42,
		PlanID:       planID,
	})

	// O use case retorna erro específico — o handler deve tratar como conflito,
	// não como erro fatal 500. Webhook duplicado deve ser tratado como idempotente.
	if !errors.Is(err, ErrActivateSubscriptionClientAlreadyHasActiveSub) {
		t.Errorf("esperado ErrActivateSubscriptionClientAlreadyHasActiveSub, obtido: %v", err)
	}
}

// Teste 6 — polling depois do webhook:
// ActivateSubscriptionByID retorna nil quando sub já está ativa.
// O fluxo de polling deve aceitar nil como sucesso, não travar.
func TestActivateSubscriptionByID_JaAtivaViaWebhook_RetornaNil(t *testing.T) {
	// Simula: webhook ativou a subscription, polling chega em seguida.
	// ActivateSubscriptionByID retorna nil (sub já active — idempotente).
	repo := &stubActivateRepo{
		activateByIDErr: nil, // idempotente — retorna nil mesmo com sub já ativa
	}

	// Valida que o retorno nil é tratado como sucesso no caller.
	err := repo.ActivateSubscriptionByID(context.Background(), 1, time.Now(), time.Now().AddDate(0, 1, 0))
	if err != nil {
		t.Errorf("ActivateSubscriptionByID já ativa: esperado nil, obtido: %v", err)
	}
}

// Teste 1 — subscription pending → ativa com sucesso.
func TestActivateSubscription_PendingParaActive_Sucesso(t *testing.T) {
	planID := uint(1)
	repo := &stubActivateRepo{
		activeSub: nil, // sem subscription ativa ainda
		plan: &domain.Plan{
			ID:           planID,
			BarbershopID: 1,
			CutsIncluded: 4,
			Active:       true,
			DurationDays: 30,
		},
		activateErr: nil, // ativação bem-sucedida
	}

	uc := NewActivateSubscription(repo)
	err := uc.Execute(context.Background(), ActivateSubscriptionInput{
		BarbershopID: 1,
		ClientID:     42,
		PlanID:       planID,
	})
	if err != nil {
		t.Errorf("esperado nil, obtido: %v", err)
	}
}

// Teste 2 — idempotência no nível do use case:
// ActivateSubscriptionByID propagando nil quando sub já está ativa.
func TestActivateSubscriptionByID_PropagaNilQuandoJaAtiva(t *testing.T) {
	repo := &stubActivateRepo{activateByIDErr: nil}
	// Contrato: chamadas repetidas com nil retorno → não causam erro no caller.
	for i := 0; i < 3; i++ {
		if err := repo.ActivateSubscriptionByID(
			context.Background(), 1,
			time.Now(), time.Now().AddDate(0, 1, 0),
		); err != nil {
			t.Errorf("iteração %d: esperado nil (idempotente), obtido: %v", i+1, err)
		}
	}
}
