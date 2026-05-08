package pagbank

import (
	"context"
	"testing"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
)

func TestGateway_WebhookPath(t *testing.T) {
	g := &Gateway{}
	const want = "/api/webhooks/pagbank"
	if got := g.WebhookPath(); got != want {
		t.Errorf("WebhookPath() = %q, want %q", got, want)
	}
}

// TestGateway_WebhookPath_ImplementsInterface garante que Gateway satisfaz a
// interface local webhookPather usada em create_transparent_payment.
func TestGateway_WebhookPath_ImplementsInterface(t *testing.T) {
	type webhookPather interface{ WebhookPath() string }
	var _ webhookPather = (*Gateway)(nil)
}

// TestGateway_ImplementsStatusChecker garante que *Gateway satisfaz a interface
// statusChecker usada em CheckPaymentStatus para polling genérico de providers.
func TestGateway_ImplementsStatusChecker(t *testing.T) {
	type statusChecker interface {
		GetPaymentStatus(ctx context.Context, providerPaymentID string) (domain.ProviderPaymentStatus, error)
	}
	var _ statusChecker = (*Gateway)(nil)
}

func TestGateway_ProviderName(t *testing.T) {
	g := &Gateway{}
	const want = "pagbank"
	if got := g.ProviderName(); got != want {
		t.Errorf("ProviderName() = %q, want %q", got, want)
	}
}

// TestGateway_ImplementsProviderNamer garante que *Gateway satisfaz a interface
// providerNamer usada em create_transparent_payment para gravar o campo provider.
func TestGateway_ImplementsProviderNamer(t *testing.T) {
	type providerNamer interface{ ProviderName() string }
	var _ providerNamer = (*Gateway)(nil)
}
