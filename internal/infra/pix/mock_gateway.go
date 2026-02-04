package pix

import (
	"fmt"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
)

type MockPixGateway struct{}

func NewMockPixGateway() *MockPixGateway {
	return &MockPixGateway{}
}

func (g *MockPixGateway) CreateCharge(
	amount float64,
	description string,
) (*domain.PixCharge, error) {

	now := time.Now().UTC()

	return &domain.PixCharge{
		TxID:      fmt.Sprintf("tx_%d", now.UnixNano()),
		QRCode:    fmt.Sprintf("PIX_QR_CODE_%d", now.Unix()),
		ExpiresAt: now.Add(15 * time.Minute),
	}, nil
}
