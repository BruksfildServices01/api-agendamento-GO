package dto

import "time"

type PixResponse struct {
	PaymentID uint       `json:"payment_id"`
	Pix       PixPayload `json:"pix"`
}

type PixPayload struct {
	TxID      string    `json:"txid"`
	QRCode    string    `json:"qr_code"`
	ExpiresAt time.Time `json:"expires_at"`
}
