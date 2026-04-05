package dto

type PublicCartItemDTO struct {
	ProductID      uint   `json:"product_id"`
	ProductName    string `json:"product_name"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	LineTotalCents int64  `json:"line_total_cents"`
}

type PublicCartDTO struct {
	Key           string              `json:"key"`
	Items         []PublicCartItemDTO `json:"items"`
	SubtotalCents int64               `json:"subtotal_cents"`
	TotalCents    int64               `json:"total_cents"`
}
