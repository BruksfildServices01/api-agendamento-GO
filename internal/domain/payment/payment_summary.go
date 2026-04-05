package payment

type PaymentSummary struct {
	TotalPaid    int64 `json:"total_paid"`
	TotalPending int64 `json:"total_pending"`
	TotalExpired int64 `json:"total_expired"`

	CountPaid    int64 `json:"count_paid"`
	CountPending int64 `json:"count_pending"`
	CountExpired int64 `json:"count_expired"`
}
