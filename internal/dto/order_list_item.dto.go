package dto

import "time"

type OrderListItemDTO struct {
	ID          uint      `json:"id"`
	Status      string    `json:"status"`
	OrderSource string    `json:"order_source"` // "suggestion" | "standalone"
	ItemsCount  int       `json:"items_count"`
	TotalCents  int64     `json:"total_cents"`
	ClientName  string    `json:"client_name,omitempty"`
	ServiceName string    `json:"service_name,omitempty"` // set when order_source = "suggestion"
	CreatedAt   time.Time `json:"created_at"`
}
