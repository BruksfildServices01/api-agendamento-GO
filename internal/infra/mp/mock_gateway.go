package mp

import (
	"fmt"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
)

// MockGateway é um gateway fake para testes e desenvolvimento.
// Retorna respostas fictícias sem chamar a API real.
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

func (g *MockGateway) CreatePayment(input domain.TransparentPaymentInput) (*domain.TransparentPaymentResult, error) {
	fakeID := time.Now().UnixNano()

	if input.PaymentMethodID == "pix" {
		fakeQR := fmt.Sprintf("00020101021226830014BR.GOV.BCB.PIX0114mock%d5204000053039865802BR5925Mock Barbearia6009Sao Paulo62290525mock-pix-%d6304ABCD", fakeID, fakeID)
		return &domain.TransparentPaymentResult{
			MPPaymentID:  fakeID,
			Status:       "pending",
			StatusDetail: "waiting_transfer",
			QRCode:       fakeQR,
			QRCodeBase64: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
			TicketURL:    fmt.Sprintf("https://sandbox.mercadopago.com.br/sandbox/payments/%d/ticket", fakeID),
		}, nil
	}

	// Cartão: simula aprovação imediata
	return &domain.TransparentPaymentResult{
		MPPaymentID:  fakeID,
		Status:       "approved",
		StatusDetail: "accredited",
	}, nil
}
