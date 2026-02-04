package payment

import "time"

type PixCharge struct {
	TxID      string
	QRCode    string
	ExpiresAt time.Time
}

type PixGateway interface {
	CreateCharge(
		amount float64,
		description string,
	) (*PixCharge, error)
}
