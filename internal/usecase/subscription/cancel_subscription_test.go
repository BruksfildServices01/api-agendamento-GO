package subscription

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

// ── stub do repositório para testes de cancelamento ─────────────────────────────

type stubCancelRepo struct {
	cancelErr error
}

func (r *stubCancelRepo) CancelSubscription(_ context.Context, _, _ uint) error {
	return r.cancelErr
}

// Métodos restantes — não exercitados nestes testes.
func (r *stubCancelRepo) CreatePlan(_ context.Context, _ *domain.Plan, _, _ []uint) error {
	return nil
}
func (r *stubCancelRepo) UpdatePlan(_ context.Context, _, _ uint, _ *domain.Plan, _, _ []uint) error {
	return nil
}
func (r *stubCancelRepo) SetPlanActive(_ context.Context, _, _ uint, _ bool) error { return nil }
func (r *stubCancelRepo) ListPlans(_ context.Context, _ uint) ([]domain.Plan, error) {
	return nil, nil
}
func (r *stubCancelRepo) GetPlanByID(_ context.Context, _, _ uint) (*domain.Plan, error) {
	return nil, nil
}
func (r *stubCancelRepo) DeletePlan(_ context.Context, _, _ uint) error { return nil }
func (r *stubCancelRepo) CountActiveSubscriptionsByPlan(_ context.Context, _ uint) (int64, error) {
	return 0, nil
}
func (r *stubCancelRepo) CountActiveSubscribersByPlan(_ context.Context, _ uint) (int64, error) {
	return 0, nil
}
func (r *stubCancelRepo) ActivateSubscription(_ context.Context, _ *domain.Subscription) error {
	return nil
}
func (r *stubCancelRepo) GetActiveSubscription(_ context.Context, _, _ uint) (*domain.Subscription, error) {
	return nil, nil
}
func (r *stubCancelRepo) CreatePendingSubscription(_ context.Context, _ *domain.Subscription) error {
	return nil
}
func (r *stubCancelRepo) GetSubscriptionByID(_ context.Context, _ uint) (*domain.Subscription, error) {
	return nil, nil
}
func (r *stubCancelRepo) ActivateSubscriptionByID(_ context.Context, _ uint, _, _ time.Time) error {
	return nil
}
func (r *stubCancelRepo) ExpireSubscriptions(_ context.Context) (int64, error) { return 0, nil }
func (r *stubCancelRepo) IncrementCutsUsed(_ context.Context, _, _ uint) error { return nil }
func (r *stubCancelRepo) ReserveSubscriptionCut(_ context.Context, _, _ uint) error { return nil }
func (r *stubCancelRepo) ReleaseSubscriptionCut(_ context.Context, _, _ uint) error { return nil }
func (r *stubCancelRepo) ConsumeReservedCut(_ context.Context, _, _ uint) error  { return nil }
func (r *stubCancelRepo) AddServiceToPlan(_ context.Context, _, _ uint) error    { return nil }
func (r *stubCancelRepo) ListAllowedServiceIDs(_ context.Context, _ uint) ([]uint, error) {
	return nil, nil
}
func (r *stubCancelRepo) UpdateCutsUsed(_ context.Context, _ uint, _ int) error { return nil }
func (r *stubCancelRepo) CountServicesByIDs(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}
func (r *stubCancelRepo) CountServicesByBarbershop(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}
func (r *stubCancelRepo) CountCategoriesByIDs(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}

// ── testes do use case CancelSubscription ────────────────────────────────────────

// Caso 1 — cancelar assinatura ativa sem reservas: repo retorna nil → use case retorna nil.
func TestCancelSubscription_SemReservas_Sucesso(t *testing.T) {
	uc := NewCancelSubscription(&stubCancelRepo{cancelErr: nil})
	if err := uc.Execute(context.Background(), 1, 42); err != nil {
		t.Errorf("esperado nil, obtido: %v", err)
	}
}

// Caso 2 — cancelar assinatura com reservas: o repo (com transação real) aplica tudo em
// conjunto. No nível do use case, sucesso do repo → sucesso do use case.
func TestCancelSubscription_ComReservas_SuccessoPropagado(t *testing.T) {
	uc := NewCancelSubscription(&stubCancelRepo{cancelErr: nil})
	if err := uc.Execute(context.Background(), 1, 42); err != nil {
		t.Errorf("esperado nil (repo aplicou transação), obtido: %v", err)
	}
}

// Caso 5 — cancelar assinatura inexistente ou já cancelada.
// O repo retorna ErrActiveSubscriptionNotFound; o use case deve repassar esse erro.
func TestCancelSubscription_SemAssinatura_RetornaNotFound(t *testing.T) {
	uc := NewCancelSubscription(&stubCancelRepo{cancelErr: domain.ErrActiveSubscriptionNotFound})
	err := uc.Execute(context.Background(), 1, 42)
	if !errors.Is(err, ErrActiveSubscriptionNotFound) {
		t.Errorf("esperado ErrActiveSubscriptionNotFound, obtido: %v", err)
	}
}

// Validação de input — barbershopID zero deve rejeitar antes de chamar o repo.
func TestCancelSubscription_BarbershopIDZero_ErrInvalidInput(t *testing.T) {
	uc := NewCancelSubscription(&stubCancelRepo{cancelErr: nil})
	err := uc.Execute(context.Background(), 0, 42)
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("esperado ErrInvalidInput para barbershopID=0, obtido: %v", err)
	}
}

// Validação de input — clientID zero deve rejeitar antes de chamar o repo.
func TestCancelSubscription_ClientIDZero_ErrInvalidInput(t *testing.T) {
	uc := NewCancelSubscription(&stubCancelRepo{cancelErr: nil})
	err := uc.Execute(context.Background(), 1, 0)
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("esperado ErrInvalidInput para clientID=0, obtido: %v", err)
	}
}

// Caso 6 — erro interno do repo (ex: falha ao atualizar appointments na transação)
// deve ser propagado sem mascaramento.
func TestCancelSubscription_ErroDoRepo_Propagado(t *testing.T) {
	repoErr := errors.New("db_connection_lost")
	uc := NewCancelSubscription(&stubCancelRepo{cancelErr: repoErr})
	err := uc.Execute(context.Background(), 1, 42)
	if err == nil || err.Error() != repoErr.Error() {
		t.Errorf("esperado erro %q, obtido: %v", repoErr, err)
	}
}

// Nota sobre testes transacionais (integração):
//
// Os testes acima verificam o contrato do use case. A garantia transacional
// (cancelar assinatura + zerar cuts_reserved + limpar appointments futuros numa
// única transação) está no repositório (subscription_gorm.go → CancelSubscription)
// e requer um banco PostgreSQL de teste para ser verificada.
//
// Comportamentos específicos que precisam de teste de integração:
// - Appointments futuros (scheduled/awaiting_payment) ficam com reserved_subscription_cut=false
// - Appointments passados/completed não são alterados
// - Appointments cancelled/no_show não são alterados
// - Se o UPDATE de appointments falhar, o cancelamento da subscription é revertido
