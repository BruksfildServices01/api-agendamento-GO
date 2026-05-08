package payment

import (
	"testing"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
)

// webhookPather espelha a interface local definida em create_transparent_payment.go.
type webhookPather interface{ WebhookPath() string }

// gatewayWithPath é um stub mínimo que implementa TransparentGateway + WebhookPath + ProviderName.
type gatewayWithPath struct {
	path         string
	providerName string
}

func (g *gatewayWithPath) CreatePayment(_ domain.TransparentPaymentInput) (*domain.TransparentPaymentResult, error) {
	return nil, nil
}
func (g *gatewayWithPath) WebhookPath() string    { return g.path }
func (g *gatewayWithPath) ProviderName() string   { return g.providerName }

// gatewayWithoutPath é um stub que só implementa TransparentGateway (sem WebhookPath).
type gatewayWithoutPath struct{}

func (g *gatewayWithoutPath) CreatePayment(_ domain.TransparentPaymentInput) (*domain.TransparentPaymentResult, error) {
	return nil, nil
}

// resolveWebhookPathFor replica a lógica inline de create_transparent_payment.go
// para que possamos testá-la sem instanciar o use case completo.
func resolveWebhookPathFor(gw domain.TransparentGateway) string {
	if wp, ok := gw.(webhookPather); ok {
		return wp.WebhookPath()
	}
	return "/api/webhooks/mp"
}

func TestResolveWebhookPath_PagBankGateway(t *testing.T) {
	gw := &gatewayWithPath{path: "/api/webhooks/pagbank"}
	got := resolveWebhookPathFor(gw)
	if got != "/api/webhooks/pagbank" {
		t.Errorf("resolveWebhookPathFor(pagbank) = %q, want %q", got, "/api/webhooks/pagbank")
	}
}

func TestResolveWebhookPath_MPGatewayFallback(t *testing.T) {
	gw := &gatewayWithoutPath{}
	got := resolveWebhookPathFor(gw)
	if got != "/api/webhooks/mp" {
		t.Errorf("resolveWebhookPathFor(mp fallback) = %q, want %q", got, "/api/webhooks/mp")
	}
}

func TestResolveWebhookPath_CustomPath(t *testing.T) {
	gw := &gatewayWithPath{path: "/api/webhooks/custom"}
	got := resolveWebhookPathFor(gw)
	if got != "/api/webhooks/custom" {
		t.Errorf("resolveWebhookPathFor(custom) = %q, want %q", got, "/api/webhooks/custom")
	}
}

// ── providerNamer: gravar provider e provider_payment_id ─────────────────────────

// providerNamer espelha a interface local de create_transparent_payment.go.
type providerNamer interface{ ProviderName() string }

// resolveProviderName replica a lógica inline de create_transparent_payment.go.
func resolveProviderName(gw domain.TransparentGateway) string {
	if pn, ok := gw.(providerNamer); ok {
		return pn.ProviderName()
	}
	return ""
}

// resolveRawProviderID replica a lógica inline de create_transparent_payment.go.
// Retira o prefixo "mp_pay:" do TxID para obter o ID puro.
func resolveRawProviderID(txid string) string {
	const prefix = "mp_pay:"
	if len(txid) > len(prefix) && txid[:len(prefix)] == prefix {
		return txid[len(prefix):]
	}
	return txid
}

func TestProviderNamer_PagBankGateway(t *testing.T) {
	gw := &gatewayWithPath{providerName: "pagbank"}
	got := resolveProviderName(gw)
	if got != "pagbank" {
		t.Errorf("resolveProviderName(pagbank) = %q, want %q", got, "pagbank")
	}
}

func TestProviderNamer_MPGatewayFallback(t *testing.T) {
	gw := &gatewayWithoutPath{}
	got := resolveProviderName(gw)
	if got != "" {
		t.Errorf("resolveProviderName(gateway sem ProviderName) deve ser vazio, got %q", got)
	}
}

func TestResolveRawProviderID_MPTxIDStripsPrefix(t *testing.T) {
	got := resolveRawProviderID("mp_pay:12345")
	if got != "12345" {
		t.Errorf("deve remover prefixo mp_pay:, got %q", got)
	}
}

func TestResolveRawProviderID_PagBankUnchanged(t *testing.T) {
	got := resolveRawProviderID("QRC_ABCDEF")
	if got != "QRC_ABCDEF" {
		t.Errorf("QRC_ não deve ser modificado, got %q", got)
	}
}

func TestResolveRawProviderID_ChargeUnchanged(t *testing.T) {
	got := resolveRawProviderID("CHAR_ABCDEF")
	if got != "CHAR_ABCDEF" {
		t.Errorf("CHAR_ não deve ser modificado, got %q", got)
	}
}
