package dto

type PublicOrchestratedCheckoutRequestDTO struct {
	ServiceID      uint    `json:"service_id" binding:"required"`
	Date           string  `json:"date" binding:"required"` // YYYY-MM-DD
	Time           string  `json:"time" binding:"required"` // HH:mm
	ClientName     string  `json:"client_name" binding:"required"`
	ClientPhone    string  `json:"client_phone" binding:"required"`
	ClientEmail    string  `json:"client_email"`
	Notes          string  `json:"notes"`
	CartKey        *string `json:"cart_key,omitempty"`
	IdempotencyKey string  `json:"-"`
}
