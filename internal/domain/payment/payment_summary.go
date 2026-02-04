package payment

type PaymentSummary struct {
	TotalPaid    float64 `json:"total_paid"`
	TotalPending float64 `json:"total_pending"`
	TotalExpired float64 `json:"total_expired"`

	CountPaid    int64 `json:"count_paid"`
	CountPending int64 `json:"count_pending"`
	CountExpired int64 `json:"count_expired"`
}
