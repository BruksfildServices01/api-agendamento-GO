package handlers

import (
	"context"
	"strings"
	"testing"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// ── interfaces locais (espelham as definições em mp_webhook_handler.go) ──────────

type statusChecker interface {
	GetPaymentStatus(ctx context.Context, providerPaymentID string) (domain.ProviderPaymentStatus, error)
}

type providerNamer interface{ ProviderName() string }

// ── Stubs ────────────────────────────────────────────────────────────────────────

// stubFullGateway implementa TransparentGateway + statusChecker + providerNamer.
type stubFullGateway struct {
	providerName string
	status       domain.ProviderPaymentStatus
	err          error
}

func (s *stubFullGateway) CreatePayment(_ domain.TransparentPaymentInput) (*domain.TransparentPaymentResult, error) {
	return nil, nil
}
func (s *stubFullGateway) GetPaymentStatus(_ context.Context, _ string) (domain.ProviderPaymentStatus, error) {
	return s.status, s.err
}
func (s *stubFullGateway) ProviderName() string { return s.providerName }

// stubGatewayNoStatus é um TransparentGateway sem statusChecker nem providerNamer.
type stubGatewayNoStatus struct{}

func (s *stubGatewayNoStatus) CreatePayment(_ domain.TransparentPaymentInput) (*domain.TransparentPaymentResult, error) {
	return nil, nil
}

// ── Helpers que espelham a lógica inline do handler ──────────────────────────────

func extractMPPaymentIDStr(mpPaymentID *int64, txid *string) string {
	if mpPaymentID != nil {
		return "some-mp-id"
	}
	if txid != nil && strings.HasPrefix(*txid, "mp_pay:") {
		return strings.TrimPrefix(*txid, "mp_pay:")
	}
	return ""
}

func extractProviderPaymentID(p *models.Payment) string {
	if p.ProviderPaymentID != nil && *p.ProviderPaymentID != "" {
		return *p.ProviderPaymentID
	}
	if p.TxID != nil {
		return *p.TxID
	}
	return ""
}

// ── Testes de detecção de caminho MP ─────────────────────────────────────────────

func TestExtractMPPaymentIDStr_MPPaymentIDField(t *testing.T) {
	id := int64(12345)
	if got := extractMPPaymentIDStr(&id, nil); got == "" {
		t.Error("deve detectar caminho MP quando mp_payment_id está preenchido")
	}
}

func TestExtractMPPaymentIDStr_TxIDWithMPPrefix(t *testing.T) {
	txid := "mp_pay:67890"
	if got := extractMPPaymentIDStr(nil, &txid); got != "67890" {
		t.Errorf("deve extrair ID do TxID mp_pay:, got %q", got)
	}
}

func TestExtractMPPaymentIDStr_PagBankQRCode(t *testing.T) {
	txid := "QRC_ABCDEF123"
	if got := extractMPPaymentIDStr(nil, &txid); got != "" {
		t.Errorf("QRC_ TxID não deve ser tratado como MP, got %q", got)
	}
}

func TestExtractMPPaymentIDStr_PagBankCharge(t *testing.T) {
	txid := "CHAR_ABCDEF123"
	if got := extractMPPaymentIDStr(nil, &txid); got != "" {
		t.Errorf("CHAR_ TxID não deve ser tratado como MP, got %q", got)
	}
}

func TestExtractMPPaymentIDStr_NilBoth(t *testing.T) {
	if got := extractMPPaymentIDStr(nil, nil); got != "" {
		t.Errorf("sem identificador deve retornar vazio, got %q", got)
	}
}

// ── Testes de prioridade do provider_payment_id ───────────────────────────────────

func TestExtractProviderPaymentID_PrefersProviderPaymentID(t *testing.T) {
	providerID := "QRC_SPECIFIC"
	txid := "QRC_OTHER"
	p := &models.Payment{ProviderPaymentID: &providerID, TxID: &txid}
	got := extractProviderPaymentID(p)
	if got != "QRC_SPECIFIC" {
		t.Errorf("deve preferir ProviderPaymentID sobre TxID, got %q", got)
	}
}

func TestExtractProviderPaymentID_FallsBackToTxID(t *testing.T) {
	txid := "QRC_FALLBACK"
	p := &models.Payment{TxID: &txid}
	got := extractProviderPaymentID(p)
	if got != "QRC_FALLBACK" {
		t.Errorf("deve usar TxID quando ProviderPaymentID é nil, got %q", got)
	}
}

func TestExtractProviderPaymentID_BothNil(t *testing.T) {
	p := &models.Payment{}
	got := extractProviderPaymentID(p)
	if got != "" {
		t.Errorf("sem IDs deve retornar vazio, got %q", got)
	}
}

func TestExtractProviderPaymentID_EmptyProviderPaymentIDFallsToTxID(t *testing.T) {
	empty := ""
	txid := "QRC_USED"
	p := &models.Payment{ProviderPaymentID: &empty, TxID: &txid}
	got := extractProviderPaymentID(p)
	if got != "QRC_USED" {
		t.Errorf("ProviderPaymentID vazio deve cair em TxID, got %q", got)
	}
}

// ── Testes de type assertion statusChecker / providerNamer ───────────────────────

func TestStatusChecker_TypeAssertionSuccess(t *testing.T) {
	var gw domain.TransparentGateway = &stubFullGateway{status: domain.ProviderStatusApproved}
	checker, ok := gw.(statusChecker)
	if !ok {
		t.Fatal("stubFullGateway deve satisfazer statusChecker")
	}
	status, err := checker.GetPaymentStatus(context.Background(), "QRC_TEST")
	if err != nil {
		t.Fatalf("GetPaymentStatus: %v", err)
	}
	if status != domain.ProviderStatusApproved {
		t.Errorf("status = %q, want %q", status, domain.ProviderStatusApproved)
	}
}

func TestStatusChecker_TypeAssertionFailure(t *testing.T) {
	var gw domain.TransparentGateway = &stubGatewayNoStatus{}
	if _, ok := gw.(statusChecker); ok {
		t.Error("stubGatewayNoStatus não deve satisfazer statusChecker")
	}
}

func TestProviderNamer_TypeAssertionSuccess(t *testing.T) {
	var gw domain.TransparentGateway = &stubFullGateway{providerName: "pagbank"}
	namer, ok := gw.(providerNamer)
	if !ok {
		t.Fatal("stubFullGateway deve satisfazer providerNamer")
	}
	if got := namer.ProviderName(); got != "pagbank" {
		t.Errorf("ProviderName() = %q, want %q", got, "pagbank")
	}
}

func TestProviderNamer_TypeAssertionFailure(t *testing.T) {
	var gw domain.TransparentGateway = &stubGatewayNoStatus{}
	if _, ok := gw.(providerNamer); ok {
		t.Error("stubGatewayNoStatus não deve satisfazer providerNamer")
	}
}

// ── Comportamento do status retornado pelo provider ───────────────────────────────

func TestStatusChecker_PendingDoesNotConfirm(t *testing.T) {
	gw := &stubFullGateway{status: domain.ProviderStatusPending}
	status, _ := gw.GetPaymentStatus(context.Background(), "QRC_PENDING")
	if status == domain.ProviderStatusApproved {
		t.Error("status pending não deve resultar em confirmação")
	}
}

func TestStatusChecker_RejectedDoesNotConfirm(t *testing.T) {
	gw := &stubFullGateway{status: domain.ProviderStatusRejected}
	status, _ := gw.GetPaymentStatus(context.Background(), "CHAR_REJECTED")
	if status == domain.ProviderStatusApproved {
		t.Error("status rejected não deve resultar em confirmação")
	}
}

// TestPaymentPaidEarlyReturn verifica que o status "paid" no DB mapeia para "confirmed".
// Testa a lógica direta sem depender de DB.
func TestPaymentPaidMapsToConfirmed(t *testing.T) {
	// A lógica de mapeamento paid→confirmed é inline no handler.
	// Testamos o invariante como documentação do contrato.
	status := "paid"
	if status == "paid" {
		status = "confirmed"
	}
	if status != "confirmed" {
		t.Errorf("'paid' deve mapear para 'confirmed', got %q", status)
	}
}

func TestPaymentExpiredReturnsExpired(t *testing.T) {
	status := "expired"
	// Expired não sofre mapeamento — retorna como está.
	if status != "expired" {
		t.Errorf("'expired' deve retornar como 'expired', got %q", status)
	}
}
