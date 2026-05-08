package subscription

import (
	"strings"
	"testing"

	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
)

// ── interfaces locais (espelham as definições inline em purchase_subscription.go) ─

type webhookPather interface{ WebhookPath() string }
type providerNamer interface{ ProviderName() string }

// ── stubs ─────────────────────────────────────────────────────────────────────────

type stubGatewayFull struct {
	webhookPath  string
	providerName string
}

func (g *stubGatewayFull) CreatePayment(_ domainPayment.TransparentPaymentInput) (*domainPayment.TransparentPaymentResult, error) {
	return &domainPayment.TransparentPaymentResult{Status: "pending"}, nil
}
func (g *stubGatewayFull) WebhookPath() string   { return g.webhookPath }
func (g *stubGatewayFull) ProviderName() string  { return g.providerName }

type stubGatewayNoExtras struct{}

func (g *stubGatewayNoExtras) CreatePayment(_ domainPayment.TransparentPaymentInput) (*domainPayment.TransparentPaymentResult, error) {
	return &domainPayment.TransparentPaymentResult{Status: "pending"}, nil
}

// ── notification URL por provider ─────────────────────────────────────────────────

// resolveNotifURL replica a lógica inline de purchase_subscription.go.
func resolveNotifURL(backendURL string, gw domainPayment.TransparentGateway) string {
	if strings.Contains(backendURL, "localhost") || strings.Contains(backendURL, "127.0.0.1") {
		return ""
	}
	webhookPath := "/api/webhooks/mp"
	if wp, ok := gw.(webhookPather); ok {
		webhookPath = wp.WebhookPath()
	}
	return strings.TrimRight(backendURL, "/") + webhookPath
}

func TestNotifURL_PagBank_UsesCorrectPath(t *testing.T) {
	gw := &stubGatewayFull{webhookPath: "/api/webhooks/pagbank"}
	got := resolveNotifURL("https://api.example.com", gw)
	if got != "https://api.example.com/api/webhooks/pagbank" {
		t.Errorf("PagBank notifURL = %q", got)
	}
}

func TestNotifURL_MP_UsesCorrectPath(t *testing.T) {
	gw := &stubGatewayFull{webhookPath: "/api/webhooks/mp"}
	got := resolveNotifURL("https://api.example.com", gw)
	if got != "https://api.example.com/api/webhooks/mp" {
		t.Errorf("MP notifURL = %q", got)
	}
}

func TestNotifURL_GatewayWithoutWebhookPath_FallsBackToMP(t *testing.T) {
	gw := &stubGatewayNoExtras{}
	got := resolveNotifURL("https://api.example.com", gw)
	if got != "https://api.example.com/api/webhooks/mp" {
		t.Errorf("fallback notifURL = %q", got)
	}
}

func TestNotifURL_Localhost_IsEmpty(t *testing.T) {
	gw := &stubGatewayFull{webhookPath: "/api/webhooks/pagbank"}
	if got := resolveNotifURL("http://localhost:8080", gw); got != "" {
		t.Errorf("localhost notifURL deve ser vazio, got %q", got)
	}
	if got := resolveNotifURL("http://127.0.0.1:8080", gw); got != "" {
		t.Errorf("127.0.0.1 notifURL deve ser vazio, got %q", got)
	}
}

// ── gravação de provider e provider_payment_id ────────────────────────────────────

// resolveProviderName replica a lógica inline de purchase_subscription.go.
func resolveProviderName(gw domainPayment.TransparentGateway) string {
	if pn, ok := gw.(providerNamer); ok {
		return pn.ProviderName()
	}
	return ""
}

// resolveRawID replica a lógica inline de purchase_subscription.go.
func resolveRawID(result *domainPayment.TransparentPaymentResult) string {
	const mpPayPfx = "mp_pay:"
	if result.ProviderPaymentID != "" {
		return strings.TrimPrefix(result.ProviderPaymentID, mpPayPfx)
	}
	if result.MPPaymentID != 0 {
		return ""  // sem strconv aqui: só testa a lógica de escolha de campo
	}
	return ""
}

func TestProviderName_PagBank(t *testing.T) {
	gw := &stubGatewayFull{providerName: "pagbank"}
	if got := resolveProviderName(gw); got != "pagbank" {
		t.Errorf("ProviderName PagBank = %q", got)
	}
}

func TestProviderName_MP(t *testing.T) {
	gw := &stubGatewayFull{providerName: "mercadopago"}
	if got := resolveProviderName(gw); got != "mercadopago" {
		t.Errorf("ProviderName MP = %q", got)
	}
}

func TestProviderName_GatewayWithoutNamer_Empty(t *testing.T) {
	gw := &stubGatewayNoExtras{}
	if got := resolveProviderName(gw); got != "" {
		t.Errorf("gateway sem ProviderName deve retornar vazio, got %q", got)
	}
}

func TestRawID_PagBankQRCode(t *testing.T) {
	result := &domainPayment.TransparentPaymentResult{ProviderPaymentID: "QRC_ABCDEF"}
	got := resolveRawID(result)
	if got != "QRC_ABCDEF" {
		t.Errorf("QRC_ deve ser mantido sem alteração, got %q", got)
	}
}

func TestRawID_MPStripsPrefix(t *testing.T) {
	result := &domainPayment.TransparentPaymentResult{ProviderPaymentID: "mp_pay:12345"}
	got := resolveRawID(result)
	if got != "12345" {
		t.Errorf("mp_pay: prefix deve ser removido, got %q", got)
	}
}

func TestRawID_NoProviderID_Empty(t *testing.T) {
	result := &domainPayment.TransparentPaymentResult{}
	got := resolveRawID(result)
	if got != "" {
		t.Errorf("sem IDs deve retornar vazio, got %q", got)
	}
}
