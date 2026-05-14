package subscription

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

// stubExpireRepo implementa domain.Repository com comportamento configurável
// apenas em ExpireSubscriptions. Todos os outros métodos são no-ops.
type stubExpireRepo struct {
	expireCount int64
	expireErr   error
}

func (r *stubExpireRepo) ExpireSubscriptions(_ context.Context) (int64, error) {
	return r.expireCount, r.expireErr
}

func (r *stubExpireRepo) CreatePlan(_ context.Context, _ *domain.Plan, _, _ []uint) error {
	return nil
}
func (r *stubExpireRepo) UpdatePlan(_ context.Context, _, _ uint, _ *domain.Plan, _, _ []uint) error {
	return nil
}
func (r *stubExpireRepo) SetPlanActive(_ context.Context, _, _ uint, _ bool) error { return nil }
func (r *stubExpireRepo) ListPlans(_ context.Context, _ uint) ([]domain.Plan, error) {
	return nil, nil
}
func (r *stubExpireRepo) GetPlanByID(_ context.Context, _, _ uint) (*domain.Plan, error) {
	return nil, nil
}
func (r *stubExpireRepo) DeletePlan(_ context.Context, _, _ uint) error { return nil }
func (r *stubExpireRepo) CountActiveSubscriptionsByPlan(_ context.Context, _ uint) (int64, error) {
	return 0, nil
}
func (r *stubExpireRepo) CountActiveSubscribersByPlan(_ context.Context, _ uint) (int64, error) {
	return 0, nil
}
func (r *stubExpireRepo) ActivateSubscription(_ context.Context, _ *domain.Subscription) error {
	return nil
}
func (r *stubExpireRepo) CancelSubscription(_ context.Context, _, _ uint) error { return nil }
func (r *stubExpireRepo) GetActiveSubscription(_ context.Context, _, _ uint) (*domain.Subscription, error) {
	return nil, nil
}
func (r *stubExpireRepo) CreatePendingSubscription(_ context.Context, _ *domain.Subscription) error {
	return nil
}
func (r *stubExpireRepo) GetSubscriptionByID(_ context.Context, _ uint) (*domain.Subscription, error) {
	return nil, nil
}
func (r *stubExpireRepo) ActivateSubscriptionByID(_ context.Context, _ uint, _, _ time.Time) error {
	return nil
}
func (r *stubExpireRepo) IncrementCutsUsed(_ context.Context, _, _ uint) error { return nil }
func (r *stubExpireRepo) ReserveSubscriptionCut(_ context.Context, _, _ uint) error { return nil }
func (r *stubExpireRepo) ReleaseSubscriptionCut(_ context.Context, _, _ uint) error { return nil }
func (r *stubExpireRepo) ConsumeReservedCut(_ context.Context, _, _ uint) error  { return nil }
func (r *stubExpireRepo) AddServiceToPlan(_ context.Context, _, _ uint) error    { return nil }
func (r *stubExpireRepo) ListAllowedServiceIDs(_ context.Context, _ uint) ([]uint, error) {
	return nil, nil
}
func (r *stubExpireRepo) UpdateCutsUsed(_ context.Context, _ uint, _ int) error { return nil }
func (r *stubExpireRepo) CountServicesByIDs(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}
func (r *stubExpireRepo) CountServicesByBarbershop(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}
func (r *stubExpireRepo) CountCategoriesByIDs(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}

// ── testes do use case ───────────────────────────────────────────────────────────

// Sucesso: retorna a contagem propagada pelo repo.
func TestExpireSubscriptions_PropagaContagem(t *testing.T) {
	uc := NewExpireSubscriptions(&stubExpireRepo{expireCount: 3, expireErr: nil})
	n, err := uc.Execute(context.Background())
	if err != nil {
		t.Errorf("esperado nil, obtido: %v", err)
	}
	if n != 3 {
		t.Errorf("esperado contagem 3, obtido %d", n)
	}
}

// Sem expiradas: retorna 0 sem erro.
func TestExpireSubscriptions_ZeroExpiradas(t *testing.T) {
	uc := NewExpireSubscriptions(&stubExpireRepo{expireCount: 0, expireErr: nil})
	n, err := uc.Execute(context.Background())
	if err != nil {
		t.Errorf("esperado nil, obtido: %v", err)
	}
	if n != 0 {
		t.Errorf("esperado 0, obtido %d", n)
	}
}

// Erro do repo é propagado sem transformação.
func TestExpireSubscriptions_PropagaErro(t *testing.T) {
	repoErr := errors.New("db_timeout")
	uc := NewExpireSubscriptions(&stubExpireRepo{expireErr: repoErr})
	_, err := uc.Execute(context.Background())
	if err == nil || err.Error() != repoErr.Error() {
		t.Errorf("esperado erro %q, obtido: %v", repoErr, err)
	}
}
