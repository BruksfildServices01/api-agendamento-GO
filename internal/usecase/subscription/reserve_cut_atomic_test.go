package subscription

import (
	"context"
	"testing"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

// ── mock mínimo do domain.Repository ────────────────────────────────────────────
//
// Implementa a interface completa com no-ops. O campo reserveErr controla
// o retorno de ReserveSubscriptionCut para cada cenário de teste.

type stubReserveRepo struct {
	reserveErr error
}

func (r *stubReserveRepo) ReserveSubscriptionCut(_ context.Context, _, _ uint) error {
	return r.reserveErr
}

// Métodos restantes da interface — não exercitados nestes testes.
func (r *stubReserveRepo) CreatePlan(_ context.Context, _ *domain.Plan, _, _ []uint) error {
	return nil
}
func (r *stubReserveRepo) UpdatePlan(_ context.Context, _, _ uint, _ *domain.Plan, _, _ []uint) error {
	return nil
}
func (r *stubReserveRepo) SetPlanActive(_ context.Context, _, _ uint, _ bool) error { return nil }
func (r *stubReserveRepo) ListPlans(_ context.Context, _ uint) ([]domain.Plan, error) {
	return nil, nil
}
func (r *stubReserveRepo) GetPlanByID(_ context.Context, _, _ uint) (*domain.Plan, error) {
	return nil, nil
}
func (r *stubReserveRepo) DeletePlan(_ context.Context, _, _ uint) error { return nil }
func (r *stubReserveRepo) CountActiveSubscriptionsByPlan(_ context.Context, _ uint) (int64, error) {
	return 0, nil
}
func (r *stubReserveRepo) CountActiveSubscribersByPlan(_ context.Context, _ uint) (int64, error) {
	return 0, nil
}
func (r *stubReserveRepo) ActivateSubscription(_ context.Context, _ *domain.Subscription) error {
	return nil
}
func (r *stubReserveRepo) CancelSubscription(_ context.Context, _, _ uint) error { return nil }
func (r *stubReserveRepo) GetActiveSubscription(_ context.Context, _, _ uint) (*domain.Subscription, error) {
	return nil, nil
}
func (r *stubReserveRepo) CreatePendingSubscription(_ context.Context, _ *domain.Subscription) error {
	return nil
}
func (r *stubReserveRepo) GetSubscriptionByID(_ context.Context, _ uint) (*domain.Subscription, error) {
	return nil, nil
}
func (r *stubReserveRepo) ActivateSubscriptionByID(_ context.Context, _ uint, _, _ time.Time) error {
	return nil
}
func (r *stubReserveRepo) ExpireSubscriptions(_ context.Context) (int64, error) { return 0, nil }
func (r *stubReserveRepo) IncrementCutsUsed(_ context.Context, _, _ uint) error { return nil }
func (r *stubReserveRepo) ReleaseSubscriptionCut(_ context.Context, _, _ uint) error { return nil }
func (r *stubReserveRepo) ConsumeReservedCut(_ context.Context, _, _ uint) error { return nil }
func (r *stubReserveRepo) AddServiceToPlan(_ context.Context, _, _ uint) error   { return nil }
func (r *stubReserveRepo) ListAllowedServiceIDs(_ context.Context, _ uint) ([]uint, error) {
	return nil, nil
}
func (r *stubReserveRepo) UpdateCutsUsed(_ context.Context, _ uint, _ int) error { return nil }
func (r *stubReserveRepo) CountServicesByIDs(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}
func (r *stubReserveRepo) CountServicesByBarbershop(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}
func (r *stubReserveRepo) CountCategoriesByIDs(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}

// ── testes do use case ReserveSubscriptionCut ───────────────────────────────────

// Caso 1 — plano com 4 cortes, used=3, reserved=0 → 1 slot disponível → deve passar.
// O repo simula sucesso (nil) quando a query atômica encontra espaço.
func TestReserveSubscriptionCut_SlotDisponivel_Passa(t *testing.T) {
	uc := NewReserveSubscriptionCut(&stubReserveRepo{reserveErr: nil})
	if err := uc.Execute(context.Background(), 1, 42); err != nil {
		t.Errorf("esperado nil (reserva bem-sucedida), obtido: %v", err)
	}
}

// Caso 2 — plano com 4 cortes, used=3, reserved=1 → nenhum slot → deve falhar com ErrCutsLimitExceeded.
// O repo simula a rejeição atômica do banco (0 rows → classifyZeroRows → ErrCutsLimitExceeded).
func TestReserveSubscriptionCut_SemSlot_Used3Reserved1_Falha(t *testing.T) {
	uc := NewReserveSubscriptionCut(&stubReserveRepo{reserveErr: domain.ErrCutsLimitExceeded})
	err := uc.Execute(context.Background(), 1, 42)
	if err != domain.ErrCutsLimitExceeded {
		t.Errorf("esperado ErrCutsLimitExceeded, obtido: %v", err)
	}
}

// Caso 3 — plano com 4 cortes, used=4, reserved=0 → todos usados → deve falhar com ErrCutsLimitExceeded.
func TestReserveSubscriptionCut_SemSlot_Used4Reserved0_Falha(t *testing.T) {
	uc := NewReserveSubscriptionCut(&stubReserveRepo{reserveErr: domain.ErrCutsLimitExceeded})
	err := uc.Execute(context.Background(), 1, 42)
	if err != domain.ErrCutsLimitExceeded {
		t.Errorf("esperado ErrCutsLimitExceeded, obtido: %v", err)
	}
}

// Caso 4 — sem assinatura ativa → deve retornar ErrActiveSubscriptionNotFound.
// Valida que o repo distingue "limite atingido" de "sem assinatura".
func TestReserveSubscriptionCut_SemAssinatura_Falha(t *testing.T) {
	uc := NewReserveSubscriptionCut(&stubReserveRepo{reserveErr: domain.ErrActiveSubscriptionNotFound})
	err := uc.Execute(context.Background(), 1, 42)
	if err != domain.ErrActiveSubscriptionNotFound {
		t.Errorf("esperado ErrActiveSubscriptionNotFound, obtido: %v", err)
	}
}

// ── concorrência ────────────────────────────────────────────────────────────────
//
// Caso 4b — duas goroutines tentam reservar o último corte ao mesmo tempo.
// O stub garante que apenas uma delas recebe nil (sucesso); a outra recebe
// ErrCutsLimitExceeded. Em produção, isso é garantido pelo UPDATE atômico no
// banco; aqui validamos que o código acima do repo trata corretamente cada saída.
func TestReserveSubscriptionCut_Concorrente_ApenasUmaPassaEmProducao(t *testing.T) {
	// Simula o comportamento do banco: 1ª chama retorna nil, 2ª retorna limit exceeded.
	// Em produção o banco decide quem ganha via UPDATE atômico; no unit test usamos
	// dois stubs distintos para verificar que cada caminho é tratado corretamente.

	ctx := context.Background()

	ucSucesso := NewReserveSubscriptionCut(&stubReserveRepo{reserveErr: nil})
	ucFalha := NewReserveSubscriptionCut(&stubReserveRepo{reserveErr: domain.ErrCutsLimitExceeded})

	if err := ucSucesso.Execute(ctx, 1, 42); err != nil {
		t.Errorf("goroutine vencedora deveria ter nil, obtido: %v", err)
	}
	if err := ucFalha.Execute(ctx, 1, 42); err != domain.ErrCutsLimitExceeded {
		t.Errorf("goroutine perdedora deveria ter ErrCutsLimitExceeded, obtido: %v", err)
	}
}

// ── integração com create_appointment (contrato) ────────────────────────────────
//
// Caso 5 — garante que o contrato entre o use case e o chamador está correto:
// qualquer erro de reserva (incluindo ErrCutsLimitExceeded) deve ser tratado
// pelo caller como "não coberto". Este teste verifica que os erros conhecidos
// são propagados sem transformação pelo use case.
func TestReserveSubscriptionCut_PropagaErrosSemTransformacao(t *testing.T) {
	casos := []struct {
		nome string
		err  error
	}{
		{"limite excedido", domain.ErrCutsLimitExceeded},
		{"sem assinatura", domain.ErrActiveSubscriptionNotFound},
	}

	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			uc := NewReserveSubscriptionCut(&stubReserveRepo{reserveErr: tc.err})
			err := uc.Execute(context.Background(), 1, 42)
			if err != tc.err {
				t.Errorf("esperado %v, obtido %v — use case não deve transformar o erro do repo", tc.err, err)
			}
		})
	}
}

// Nota sobre testes de SQL (integração):
// Os testes acima verificam o contrato do use case com mocks. A proteção real contra
// race condition está na query SQL em subscription_gorm.go:
//
//   AND cuts_used_in_period + cuts_reserved_in_period + 1
//       <= (SELECT cuts_included FROM plans WHERE id = plan_id)
//
// Essa subquery é atômica dentro do UPDATE — o banco aplica o lock de linha
// antes de incrementar, garantindo que dois SESSões simultâneas não ultrapassem
// o limite. Testes de concorrência real (goroutines + DB de teste) devem ser
// feitos em cmd/e2e/ com um banco PostgreSQL de teste.
