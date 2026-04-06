package mp

import (
	"fmt"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
)

// MockGateway é um gateway fake para testes e desenvolvimento.
// Retorna uma preferência fictícia sem chamar a API real.
type MockGateway struct{}

func NewMockGateway() *MockGateway {
	return &MockGateway{}
}

func (g *MockGateway) CreatePreference(
	amountCents int64,
	description string,
	externalReference string,
	_ string,
	_ domain.MPBackURLs,
) (*domain.MPPreference, error) {
	fakeID := fmt.Sprintf("mock-pref-%d", time.Now().UnixNano())
	return &domain.MPPreference{
		PreferenceID: fakeID,
		InitPoint:    "https://sandbox.mercadopago.com.br/checkout/v1/redirect?pref_id=" + fakeID,
		SandboxPoint: "https://sandbox.mercadopago.com.br/checkout/v1/redirect?pref_id=" + fakeID,
	}, nil
}
