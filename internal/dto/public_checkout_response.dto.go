package dto

type PublicCheckoutOrderDTO struct {
	OrderID    uint   `json:"order_id"`
	Status     string `json:"status"`
	TotalCents int64  `json:"total_cents"`
	ItemsCount int    `json:"items_count"`
}

type PublicCheckoutNextStepDTO struct {
	Action     string `json:"action"`
	Method     string `json:"method"`
	PaymentURL string `json:"payment_url,omitempty"`
}

type PublicCheckoutResponseDTO struct {
	Order    PublicCheckoutOrderDTO    `json:"order"`
	NextStep PublicCheckoutNextStepDTO `json:"next_step"`
}
