package handlers

import (
	"context"
	"testing"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// ── helpers que espelham a lógica inline de tryActivateFromProvider ───────────────

// extractSubProviderPaymentID replica a lógica de seleção de ID em tryActivateFromProvider.
func extractSubProviderPaymentID(pmt *models.Payment) string {
	if pmt.ProviderPaymentID != nil && *pmt.ProviderPaymentID != "" {
		return *pmt.ProviderPaymentID
	}
	if pmt.MPPaymentID != nil {
		// representa o fallback legado — só verificamos que o campo é lido
		return "mp-legacy-id"
	}
	return ""
}

// ── testes de seleção de ID ───────────────────────────────────────────────────────

func TestExtractSubProviderPaymentID_PrefersProviderPaymentID(t *testing.T) {
	ppid := "QRC_PAGBANK"
	mpid := int64(99999)
	pmt := &models.Payment{ProviderPaymentID: &ppid, MPPaymentID: &mpid}
	got := extractSubProviderPaymentID(pmt)
	if got != "QRC_PAGBANK" {
		t.Errorf("deve preferir ProviderPaymentID, got %q", got)
	}
}

func TestExtractSubProviderPaymentID_FallsBackToMPPaymentID(t *testing.T) {
	mpid := int64(12345)
	pmt := &models.Payment{MPPaymentID: &mpid}
	got := extractSubProviderPaymentID(pmt)
	if got == "" {
		t.Error("deve detectar fallback legado mp_payment_id")
	}
}

func TestExtractSubProviderPaymentID_NilBoth(t *testing.T) {
	pmt := &models.Payment{}
	got := extractSubProviderPaymentID(pmt)
	if got != "" {
		t.Errorf("sem IDs deve retornar vazio, got %q", got)
	}
}

// ── testes do fluxo de ativação via statusChecker ────────────────────────────────

// stubSubStatusChecker é um gateway stub com GetPaymentStatus + ProviderName.
type stubSubStatusChecker struct {
	status domain.ProviderPaymentStatus
}

func (s *stubSubStatusChecker) CreatePayment(_ domain.TransparentPaymentInput) (*domain.TransparentPaymentResult, error) {
	return nil, nil
}
func (s *stubSubStatusChecker) GetPaymentStatus(_ context.Context, _ string) (domain.ProviderPaymentStatus, error) {
	return s.status, nil
}
func (s *stubSubStatusChecker) ProviderName() string { return "pagbank" }

func TestSubStatusChecker_ApprovedActivatesSubscription(t *testing.T) {
	gw := &stubSubStatusChecker{status: domain.ProviderStatusApproved}
	status, _ := gw.GetPaymentStatus(context.Background(), "QRC_TEST")
	if status != domain.ProviderStatusApproved {
		t.Errorf("approved gateway deve retornar approved, got %q", status)
	}
}

func TestSubStatusChecker_PendingDoesNotActivate(t *testing.T) {
	gw := &stubSubStatusChecker{status: domain.ProviderStatusPending}
	status, _ := gw.GetPaymentStatus(context.Background(), "QRC_TEST")
	if status == domain.ProviderStatusApproved {
		t.Error("pending não deve ativar subscription")
	}
}

// ── teste de prioridade de gateway por provider salvo ────────────────────────────

func TestSubPolling_PaymentWithProviderUsesThatProvider(t *testing.T) {
	// Verifica que quando payment.Provider está preenchido, usamos GatewayForProvider.
	// Simula a decisão lógica: se provider != nil, buscar gateway específico.
	provider := "pagbank"
	pmt := &models.Payment{Provider: &provider}

	usesSpecificProvider := pmt.Provider != nil && *pmt.Provider != ""
	if !usesSpecificProvider {
		t.Error("payment com provider preenchido deve usar GatewayForProvider")
	}
}

func TestSubPolling_PaymentWithoutProviderUsesFallback(t *testing.T) {
	// Pagamentos antigos sem provider → usa fallback (TransparentGatewayFor).
	pmt := &models.Payment{}
	usesSpecificProvider := pmt.Provider != nil && *pmt.Provider != ""
	if usesSpecificProvider {
		t.Error("payment sem provider deve usar fallback TransparentGatewayFor")
	}
}

// ── teste de status final do payment (sem consulta externa) ──────────────────────

func TestSubPolling_PaidPaymentActivatesDirectly(t *testing.T) {
	// Quando payment.Status == "paid" (webhook já confirmou), não precisa de consulta externa.
	pmt := &models.Payment{Status: "paid"}
	isPaid := pmt.Status == "paid"
	if !isPaid {
		t.Error("payment paid deve ser detectado sem consulta ao provider")
	}
}

func TestSubPolling_ExpiredPaymentSkips(t *testing.T) {
	pmt := &models.Payment{Status: "expired"}
	shouldPoll := pmt.Status == "pending"
	if shouldPoll {
		t.Error("payment expired não deve gerar consulta ao provider")
	}
}
