package subscription

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

// ── mock configurável para testes de ConsumeCut ──────────────────────────────────
//
// Permite configurar por campo o comportamento de cada método relevante.
// releaseWasCalled rastreia se ReleaseSubscriptionCut foi chamado.

type mockConsumeCutRepo struct {
	activeSub          *domain.Subscription
	activeSubErr       error
	plan               *domain.Plan
	planErr            error
	allowedServiceIDs  []uint
	allowedServicesErr error
	consumeReservedErr error
	incrementErr       error
	releaseErr         error
	releaseWasCalled   bool
}

func (r *mockConsumeCutRepo) GetActiveSubscription(_ context.Context, _, _ uint) (*domain.Subscription, error) {
	return r.activeSub, r.activeSubErr
}
func (r *mockConsumeCutRepo) GetPlanByID(_ context.Context, _, _ uint) (*domain.Plan, error) {
	return r.plan, r.planErr
}
func (r *mockConsumeCutRepo) ListAllowedServiceIDs(_ context.Context, _ uint) ([]uint, error) {
	return r.allowedServiceIDs, r.allowedServicesErr
}
func (r *mockConsumeCutRepo) ConsumeReservedCut(_ context.Context, _, _ uint) error {
	return r.consumeReservedErr
}
func (r *mockConsumeCutRepo) IncrementCutsUsed(_ context.Context, _, _ uint) error {
	return r.incrementErr
}
func (r *mockConsumeCutRepo) ReleaseSubscriptionCut(_ context.Context, _, _ uint) error {
	r.releaseWasCalled = true
	return r.releaseErr
}

// Métodos restantes — no-ops.
func (r *mockConsumeCutRepo) CreatePlan(_ context.Context, _ *domain.Plan, _, _ []uint) error {
	return nil
}
func (r *mockConsumeCutRepo) UpdatePlan(_ context.Context, _, _ uint, _ *domain.Plan, _, _ []uint) error {
	return nil
}
func (r *mockConsumeCutRepo) SetPlanActive(_ context.Context, _, _ uint, _ bool) error { return nil }
func (r *mockConsumeCutRepo) ListPlans(_ context.Context, _ uint) ([]domain.Plan, error) {
	return nil, nil
}
func (r *mockConsumeCutRepo) DeletePlan(_ context.Context, _, _ uint) error { return nil }
func (r *mockConsumeCutRepo) CountActiveSubscriptionsByPlan(_ context.Context, _ uint) (int64, error) {
	return 0, nil
}
func (r *mockConsumeCutRepo) CountActiveSubscribersByPlan(_ context.Context, _ uint) (int64, error) {
	return 0, nil
}
func (r *mockConsumeCutRepo) ActivateSubscription(_ context.Context, _ *domain.Subscription) error {
	return nil
}
func (r *mockConsumeCutRepo) CancelSubscription(_ context.Context, _, _ uint) error { return nil }
func (r *mockConsumeCutRepo) CreatePendingSubscription(_ context.Context, _ *domain.Subscription) error {
	return nil
}
func (r *mockConsumeCutRepo) GetSubscriptionByID(_ context.Context, _ uint) (*domain.Subscription, error) {
	return nil, nil
}
func (r *mockConsumeCutRepo) ActivateSubscriptionByID(_ context.Context, _ uint, _, _ time.Time) error {
	return nil
}
func (r *mockConsumeCutRepo) ExpireSubscriptions(_ context.Context) (int64, error) { return 0, nil }
func (r *mockConsumeCutRepo) ReserveSubscriptionCut(_ context.Context, _, _ uint) error { return nil }
func (r *mockConsumeCutRepo) AddServiceToPlan(_ context.Context, _, _ uint) error { return nil }
func (r *mockConsumeCutRepo) UpdateCutsUsed(_ context.Context, _ uint, _ int) error { return nil }
func (r *mockConsumeCutRepo) CountServicesByIDs(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}
func (r *mockConsumeCutRepo) CountServicesByBarbershop(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}
func (r *mockConsumeCutRepo) CountCategoriesByIDs(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────────

func validSub() *domain.Subscription {
	now := time.Now().UTC()
	planID := uint(1)
	return &domain.Subscription{
		ID:                   10,
		BarbershopID:         1,
		ClientID:             42,
		PlanID:               planID,
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     now.Add(30 * 24 * time.Hour),
		CutsUsedInPeriod:     1,
		CutsReservedInPeriod: 1,
	}
}

func activePlan() *domain.Plan {
	return &domain.Plan{
		ID:           1,
		BarbershopID: 1,
		CutsIncluded: 4,
		Active:       true,
		ServiceIDs:   []uint{5},
	}
}

// ── testes ───────────────────────────────────────────────────────────────────────

// Caso 1 — reserva válida e assinatura ativa → consome normalmente.
func TestConsumeCut_ReservaValida_Consome(t *testing.T) {
	repo := &mockConsumeCutRepo{
		activeSub:          validSub(),
		plan:               activePlan(),
		consumeReservedErr: nil, // sucesso
	}
	uc := NewConsumeCut(repo)
	result, err := uc.Execute(context.Background(), 1, 42, 5, true)
	if err != nil {
		t.Fatalf("esperado nil, obtido: %v", err)
	}
	if result.Status != ConsumeCutStatusConsumed {
		t.Errorf("esperado Consumed, obtido: %s", result.Status)
	}
	if repo.releaseWasCalled {
		t.Error("ReleaseSubscriptionCut não deve ser chamado no caminho feliz")
	}
}

// Caso 2 — reserva existente, assinatura expirada (ErrActiveSubscriptionNotFound do repo):
// não consome, libera reserva (best-effort), retorna ExpiredPeriod.
func TestConsumeCut_ReservaOrfa_AssinaturaExpirada(t *testing.T) {
	repo := &mockConsumeCutRepo{
		activeSub:          validSub(), // use case vê sub válida (race window)
		plan:               activePlan(),
		consumeReservedErr: domain.ErrActiveSubscriptionNotFound, // banco rejeita (sub expirou)
		releaseErr:         nil,
	}
	uc := NewConsumeCut(repo)
	result, err := uc.Execute(context.Background(), 1, 42, 5, true)
	if err != nil {
		t.Fatalf("esperado nil, obtido: %v", err)
	}
	if result.Status != ConsumeCutStatusExpiredPeriod {
		t.Errorf("esperado ExpiredPeriod, obtido: %s", result.Status)
	}
	if !repo.releaseWasCalled {
		t.Error("ReleaseSubscriptionCut deve ser chamado para limpar reserva órfã")
	}
}

// Caso 3 — reserva existente, assinatura cancelada (mesmo erro ErrActiveSubscriptionNotFound):
// comportamento igual ao caso de expiração.
func TestConsumeCut_ReservaOrfa_AssinaturaCancelada(t *testing.T) {
	repo := &mockConsumeCutRepo{
		activeSub:          validSub(),
		plan:               activePlan(),
		consumeReservedErr: domain.ErrActiveSubscriptionNotFound,
		releaseErr:         nil,
	}
	uc := NewConsumeCut(repo)
	result, err := uc.Execute(context.Background(), 1, 42, 5, true)
	if err != nil {
		t.Fatalf("esperado nil, obtido: %v", err)
	}
	if result.Status != ConsumeCutStatusExpiredPeriod {
		t.Errorf("esperado ExpiredPeriod (sub cancelada/expirada), obtido: %s", result.Status)
	}
	if !repo.releaseWasCalled {
		t.Error("ReleaseSubscriptionCut deve ser chamado")
	}
}

// Caso 4 — sem reserva (hadReservation=false): comportamento atual inalterado.
// ErrActiveSubscriptionNotFound do repo → NoActiveSubscription, sem release.
func TestConsumeCut_SemReserva_ComportamentoAtualPreservado(t *testing.T) {
	repo := &mockConsumeCutRepo{
		activeSub:    validSub(),
		plan:         activePlan(),
		allowedServiceIDs: []uint{5},
		incrementErr: domain.ErrActiveSubscriptionNotFound,
	}
	uc := NewConsumeCut(repo)
	result, err := uc.Execute(context.Background(), 1, 42, 5, false)
	if err != nil {
		t.Fatalf("esperado nil, obtido: %v", err)
	}
	if result.Status != ConsumeCutStatusNoActiveSubscription {
		t.Errorf("esperado NoActiveSubscription (sem reserva), obtido: %s", result.Status)
	}
	if repo.releaseWasCalled {
		t.Error("ReleaseSubscriptionCut NÃO deve ser chamado quando hadReservation=false")
	}
}

// Caso 5 — cuts_reserved_in_period não fica negativo:
// ReleaseSubscriptionCut é chamado como best-effort; se retornar erro (sub não tem reserva),
// o erro é ignorado e o status continua ExpiredPeriod.
func TestConsumeCut_ReleaseComErro_StatusPreservado(t *testing.T) {
	repo := &mockConsumeCutRepo{
		activeSub:          validSub(),
		plan:               activePlan(),
		consumeReservedErr: domain.ErrActiveSubscriptionNotFound,
		releaseErr:         errors.New("sub sem reserva ou expirada"), // release falha — é OK
	}
	uc := NewConsumeCut(repo)
	result, err := uc.Execute(context.Background(), 1, 42, 5, true)
	if err != nil {
		t.Fatalf("esperado nil mesmo com release falhando, obtido: %v", err)
	}
	if result.Status != ConsumeCutStatusExpiredPeriod {
		t.Errorf("esperado ExpiredPeriod mesmo com release falhando, obtido: %s", result.Status)
	}
	if !repo.releaseWasCalled {
		t.Error("ReleaseSubscriptionCut deve ter sido chamado (mesmo com erro)")
	}
}

// Caso 6 — integração do status retornado com o comportamento do complete.go:
// ExpiredPeriod cai em default: → requiresNormalCharging=true.
// Verificamos aqui que o mapeamento de status existe e está correto.
// O test de complete.go já cobre o path default: → normal_charging_confirmation_required.
func TestConsumeCut_ExpiredPeriod_MapeadoParaRequiresNormalCharging(t *testing.T) {
	// Este teste valida que ConsumeCutStatusExpiredPeriod é retornado corretamente
	// quando a reserva está órfã. O complete.go trata todos os status != Consumed e
	// != NoActiveSubscription com requiresNormalCharging=true (case default:).
	// Portanto, ExpiredPeriod → confirmação exigida → fechamento só com confirm=true.

	repo := &mockConsumeCutRepo{
		activeSub:          validSub(),
		plan:               activePlan(),
		consumeReservedErr: domain.ErrActiveSubscriptionNotFound,
	}
	uc := NewConsumeCut(repo)
	result, err := uc.Execute(context.Background(), 1, 42, 5, true)
	if err != nil {
		t.Fatalf("não deve retornar erro: %v", err)
	}

	// Verifica que o status NÃO é NoActiveSubscription (que fecharia sem confirmação)
	// e NÃO é Consumed.
	if result.Status == ConsumeCutStatusNoActiveSubscription {
		t.Error("status não deve ser NoActiveSubscription: fecharia sem confirmação, " +
			"mas havia reserva → barbeiro deve ser informado")
	}
	if result.Status == ConsumeCutStatusConsumed {
		t.Error("status não deve ser Consumed: não houve consumo real")
	}

	// Confirma que é ExpiredPeriod — mapeado para requiresNormalCharging=true em complete.go.
	if result.Status != ConsumeCutStatusExpiredPeriod {
		t.Errorf("esperado ExpiredPeriod para sinalizar cobrança normal confirmada, obtido: %s", result.Status)
	}
}
