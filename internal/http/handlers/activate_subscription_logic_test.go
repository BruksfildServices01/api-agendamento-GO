package handlers

// Testes de resolveNonPendingActivation.
//
// Essa função é o ponto exato da alteração em public_subscription_handler.go:
// decide o que fazer quando SELECT FOR UPDATE não encontra pending_payment.
// É pura (sem DB), testável diretamente.

import "testing"

func TestResolveNonPendingActivation_Active_ReturnsNil(t *testing.T) {
	if err := resolveNonPendingActivation(1, "active"); err != nil {
		t.Errorf("active deve retornar nil (idempotente), obtido: %v", err)
	}
}

func TestResolveNonPendingActivation_Expired_ReturnsError(t *testing.T) {
	if err := resolveNonPendingActivation(1, "expired"); err == nil {
		t.Error("expired não deve retornar nil — não é idempotência legítima")
	}
}

func TestResolveNonPendingActivation_Cancelled_ReturnsError(t *testing.T) {
	if err := resolveNonPendingActivation(1, "cancelled"); err == nil {
		t.Error("cancelled não deve retornar nil — não é idempotência legítima")
	}
}

func TestResolveNonPendingActivation_PendingPayment_ReturnsError(t *testing.T) {
	// pending_payment não deve chegar aqui (SELECT FOR UPDATE já teria encontrado),
	// mas se chegar por qualquer motivo, também não é success.
	if err := resolveNonPendingActivation(1, "pending_payment"); err == nil {
		t.Error("pending_payment não deve retornar nil neste contexto")
	}
}

func TestResolveNonPendingActivation_UnknownStatus_ReturnsError(t *testing.T) {
	if err := resolveNonPendingActivation(42, "some_other_status"); err == nil {
		t.Error("status desconhecido não deve retornar nil")
	}
}
