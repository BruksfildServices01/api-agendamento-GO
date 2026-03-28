package dto

import "time"

type OrderListItemDTO struct {
	ID            uint      `json:"id"`
	Status        string    `json:"status"`
	ItemsCount    int       `json:"items_count"`
	TotalCents    int64     `json:"total_cents"`
	PaymentStatus string    `json:"payment_status"`
	CreatedAt     time.Time `json:"created_at"`
}
