package appointment

type OperationalSummary struct {
	TotalReceived  int64 `json:"total_received"`
	CountCompleted int   `json:"count_completed"`
	CountCancelled int   `json:"count_cancelled"`
	CountNoShow    int   `json:"count_no_show"`
}
