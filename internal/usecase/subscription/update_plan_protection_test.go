package subscription

import (
	"context"
	"testing"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

// ── stub configurável para os testes de proteção do UpdatePlan ───────────────────

type stubUpdateProtectionRepo struct {
	activeSubscriberCount int64
	currentPlan           *domain.Plan
}

// CountActiveSubscribersByPlan retorna o count configurado.
func (r *stubUpdateProtectionRepo) CountActiveSubscribersByPlan(_ context.Context, _ uint) (int64, error) {
	return r.activeSubscriberCount, nil
}

// GetPlanByID retorna o plano atual configurado.
func (r *stubUpdateProtectionRepo) GetPlanByID(_ context.Context, _, _ uint) (*domain.Plan, error) {
	return r.currentPlan, nil
}

// CountServicesByBarbershop — aceita qualquer service sem erro (testes não validam isso).
func (r *stubUpdateProtectionRepo) CountServicesByBarbershop(_ context.Context, _ uint, ids []uint) (int64, error) {
	return int64(len(ids)), nil
}
func (r *stubUpdateProtectionRepo) CountCategoriesByIDs(_ context.Context, _ uint, ids []uint) (int64, error) {
	return int64(len(ids)), nil
}

// UpdatePlan é um no-op — os testes que chegam aqui devem passar.
func (r *stubUpdateProtectionRepo) UpdatePlan(_ context.Context, _, _ uint, _ *domain.Plan, _, _ []uint) error {
	return nil
}

// Métodos restantes da interface — no-ops.
func (r *stubUpdateProtectionRepo) CreatePlan(_ context.Context, _ *domain.Plan, _, _ []uint) error {
	return nil
}
func (r *stubUpdateProtectionRepo) ListPlans(_ context.Context, _ uint) ([]domain.Plan, error) {
	return nil, nil
}
func (r *stubUpdateProtectionRepo) SetPlanActive(_ context.Context, _, _ uint, _ bool) error { return nil }
func (r *stubUpdateProtectionRepo) DeletePlan(_ context.Context, _, _ uint) error             { return nil }
func (r *stubUpdateProtectionRepo) CountActiveSubscriptionsByPlan(_ context.Context, _ uint) (int64, error) {
	return 0, nil
}
func (r *stubUpdateProtectionRepo) CountActiveSubscribersByPlanByID(_ context.Context, _ uint) (int64, error) {
	return 0, nil
}
func (r *stubUpdateProtectionRepo) ActivateSubscription(_ context.Context, _ *domain.Subscription) error {
	return nil
}
func (r *stubUpdateProtectionRepo) CancelSubscription(_ context.Context, _, _ uint) error { return nil }
func (r *stubUpdateProtectionRepo) GetActiveSubscription(_ context.Context, _, _ uint) (*domain.Subscription, error) {
	return nil, nil
}
func (r *stubUpdateProtectionRepo) CreatePendingSubscription(_ context.Context, _ *domain.Subscription) error {
	return nil
}
func (r *stubUpdateProtectionRepo) GetSubscriptionByID(_ context.Context, _ uint) (*domain.Subscription, error) {
	return nil, nil
}
func (r *stubUpdateProtectionRepo) ActivateSubscriptionByID(_ context.Context, _ uint, _, _ time.Time) error {
	return nil
}
func (r *stubUpdateProtectionRepo) ExpireSubscriptions(_ context.Context) (int64, error) { return 0, nil }
func (r *stubUpdateProtectionRepo) IncrementCutsUsed(_ context.Context, _, _ uint) error  { return nil }
func (r *stubUpdateProtectionRepo) ReserveSubscriptionCut(_ context.Context, _, _ uint) error {
	return nil
}
func (r *stubUpdateProtectionRepo) ReleaseSubscriptionCut(_ context.Context, _, _ uint) error {
	return nil
}
func (r *stubUpdateProtectionRepo) ConsumeReservedCut(_ context.Context, _, _ uint) error  { return nil }
func (r *stubUpdateProtectionRepo) AddServiceToPlan(_ context.Context, _, _ uint) error    { return nil }
func (r *stubUpdateProtectionRepo) ListAllowedServiceIDs(_ context.Context, _ uint) ([]uint, error) {
	return nil, nil
}
func (r *stubUpdateProtectionRepo) UpdateCutsUsed(_ context.Context, _ uint, _ int) error { return nil }
func (r *stubUpdateProtectionRepo) CountServicesByIDs(_ context.Context, _ uint, ids []uint) (int64, error) {
	return int64(len(ids)), nil
}

// ── helpers ───────────────────────────────────────────────────────────────────────

func basePlan() *domain.Plan {
	return &domain.Plan{
		ID:                1,
		BarbershopID:      1,
		Name:              "Plano Teste",
		MonthlyPriceCents: 5000,
		DurationDays:      30,
		CutsIncluded:      4,
		DiscountPercent:   0,
		ServiceIDs:        []uint{1, 2},
		CategoryIDs:       []uint{},
	}
}

func baseInput() UpdatePlanInput {
	return UpdatePlanInput{
		BarbershopID:      1,
		PlanID:            1,
		Name:              "Plano Teste",
		MonthlyPriceCents: 5000,
		DurationDays:      30,
		CutsIncluded:      4,
		DiscountPercent:   0,
		ServiceIDs:        []uint{1, 2},
		CategoryIDs:       []uint{},
	}
}

// ── testes ───────────────────────────────────────────────────────────────────────

// Caso 1 — sem assinantes ativos: qualquer mudança é permitida.
func TestUpdatePlan_SemAssinantesAtivos_PermiteAlteracaoComercial(t *testing.T) {
	repo := &stubUpdateProtectionRepo{activeSubscriberCount: 0, currentPlan: basePlan()}
	uc := NewUpdatePlan(repo)

	input := baseInput()
	input.CutsIncluded = 2         // campo perigoso alterado
	input.MonthlyPriceCents = 9900 // campo perigoso alterado

	if err := uc.Execute(context.Background(), input); err != nil {
		t.Errorf("sem assinantes: esperado nil, obtido: %v", err)
	}
}

// Caso 2 — com assinantes ativos: bloquear alteração de cuts_included.
func TestUpdatePlan_ComAssinantesAtivos_BloqueaCutsIncluded(t *testing.T) {
	current := basePlan()
	repo := &stubUpdateProtectionRepo{activeSubscriberCount: 3, currentPlan: current}
	uc := NewUpdatePlan(repo)

	input := baseInput()
	input.CutsIncluded = 2 // reduzindo de 4 para 2

	err := uc.Execute(context.Background(), input)
	if !isErrPlanHasActiveSubscriptions(err) {
		t.Errorf("esperado ErrPlanHasActiveSubscriptions ao alterar cuts_included, obtido: %v", err)
	}
}

// Caso 3 — com assinantes ativos: bloquear alteração de monthly_price_cents.
func TestUpdatePlan_ComAssinantesAtivos_BloqueaPreco(t *testing.T) {
	repo := &stubUpdateProtectionRepo{activeSubscriberCount: 1, currentPlan: basePlan()}
	uc := NewUpdatePlan(repo)

	input := baseInput()
	input.MonthlyPriceCents = 9900 // alterando preço

	err := uc.Execute(context.Background(), input)
	if !isErrPlanHasActiveSubscriptions(err) {
		t.Errorf("esperado ErrPlanHasActiveSubscriptions ao alterar preço, obtido: %v", err)
	}
}

// Caso 4 — com assinantes ativos: bloquear alteração de duration_days.
func TestUpdatePlan_ComAssinantesAtivos_BloqueaDuracao(t *testing.T) {
	repo := &stubUpdateProtectionRepo{activeSubscriberCount: 1, currentPlan: basePlan()}
	uc := NewUpdatePlan(repo)

	input := baseInput()
	input.DurationDays = 15 // reduzindo de 30 para 15

	err := uc.Execute(context.Background(), input)
	if !isErrPlanHasActiveSubscriptions(err) {
		t.Errorf("esperado ErrPlanHasActiveSubscriptions ao alterar duração, obtido: %v", err)
	}
}

// Caso 5 — com assinantes ativos: bloquear alteração de service_ids.
func TestUpdatePlan_ComAssinantesAtivos_BloqueaServiceIDs(t *testing.T) {
	repo := &stubUpdateProtectionRepo{activeSubscriberCount: 2, currentPlan: basePlan()}
	uc := NewUpdatePlan(repo)

	input := baseInput()
	input.ServiceIDs = []uint{3} // removendo serviços cobertos

	err := uc.Execute(context.Background(), input)
	if !isErrPlanHasActiveSubscriptions(err) {
		t.Errorf("esperado ErrPlanHasActiveSubscriptions ao alterar serviceIDs, obtido: %v", err)
	}
}

// Caso 6 — com assinantes ativos: permitir alterar apenas o nome.
func TestUpdatePlan_ComAssinantesAtivos_PermiteAlterarNome(t *testing.T) {
	repo := &stubUpdateProtectionRepo{activeSubscriberCount: 5, currentPlan: basePlan()}
	uc := NewUpdatePlan(repo)

	input := baseInput()
	input.Name = "Plano Premium Renomeado" // apenas nome muda

	if err := uc.Execute(context.Background(), input); err != nil {
		t.Errorf("alterar apenas nome com assinantes ativos: esperado nil, obtido: %v", err)
	}
}

// Caso 7 — assinatura expirada/cancelada não bloqueia: count = 0.
func TestUpdatePlan_AssinaturaExpirada_NaoBloqueaEdicao(t *testing.T) {
	// count=0 simula que não há assinantes com status='active'
	repo := &stubUpdateProtectionRepo{activeSubscriberCount: 0, currentPlan: basePlan()}
	uc := NewUpdatePlan(repo)

	input := baseInput()
	input.CutsIncluded = 2
	input.MonthlyPriceCents = 100

	if err := uc.Execute(context.Background(), input); err != nil {
		t.Errorf("sem assinantes ativos (expirados/cancelados): esperado nil, obtido: %v", err)
	}
}

// ── helpers de assert ─────────────────────────────────────────────────────────────

func isErrPlanHasActiveSubscriptions(err error) bool {
	return err != nil && err.Error() == ErrPlanHasActiveSubscriptions.Error()
}

// ── testes de hasCommercialChange ────────────────────────────────────────────────

func TestHasCommercialChange_PrecoAlterado(t *testing.T) {
	current := basePlan()
	input := baseInput()
	input.MonthlyPriceCents = current.MonthlyPriceCents + 100
	if !hasCommercialChange(current, input) {
		t.Error("esperado true para preço alterado")
	}
}

func TestHasCommercialChange_DuracaoAlterada(t *testing.T) {
	current := basePlan()
	input := baseInput()
	input.DurationDays = current.DurationDays + 5
	if !hasCommercialChange(current, input) {
		t.Error("esperado true para duração alterada")
	}
}

func TestHasCommercialChange_CutsAlterado(t *testing.T) {
	current := basePlan()
	input := baseInput()
	input.CutsIncluded = current.CutsIncluded - 1
	if !hasCommercialChange(current, input) {
		t.Error("esperado true para cuts_included alterado")
	}
}

func TestHasCommercialChange_ServiceIDsAlterados(t *testing.T) {
	current := basePlan()
	input := baseInput()
	input.ServiceIDs = []uint{99}
	if !hasCommercialChange(current, input) {
		t.Error("esperado true para service_ids alterados")
	}
}

func TestHasCommercialChange_ApenasNomeAlterado_RetornaFalse(t *testing.T) {
	current := basePlan()
	input := baseInput()
	input.Name = "Novo Nome"
	if hasCommercialChange(current, input) {
		t.Error("esperado false ao alterar apenas nome")
	}
}

func TestHasCommercialChange_SemMudanca_RetornaFalse(t *testing.T) {
	current := basePlan()
	input := baseInput()
	if hasCommercialChange(current, input) {
		t.Error("esperado false quando nada muda")
	}
}

func TestHasCommercialChange_ServiceIDsOrdemDiferente_RetornaFalse(t *testing.T) {
	current := basePlan()
	input := baseInput()
	// Mesmos IDs, ordem diferente — não é mudança comercial
	input.ServiceIDs = []uint{2, 1}
	if hasCommercialChange(current, input) {
		t.Error("esperado false: mesmos serviceIDs em ordem diferente não é mudança comercial")
	}
}
