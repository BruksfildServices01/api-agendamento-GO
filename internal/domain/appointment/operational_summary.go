package appointment

type OperationalSummary struct {
	TotalReceived      int64 `json:"total_received"`
	CountCompleted     int   `json:"count_completed"`
	CountProductsSold  int   `json:"count_products_sold"`
}
