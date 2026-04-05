package payment

import "time"

type PaymentListFilter struct {
	Status    *string
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Offset    int
}
