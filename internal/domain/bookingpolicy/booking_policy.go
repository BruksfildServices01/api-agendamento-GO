package bookingpolicy

import "time"

type Category string

const (
	CategoryNew     Category = "new"
	CategoryTrusted Category = "trusted"
	CategoryRisk    Category = "risk"
	CategoryPremium Category = "premium"
)

type BookingPolicy struct {
	BarbershopID uint
	Category     Category

	RequirePrePayment   bool
	AllowPayLater       bool
	AllowManualOverride bool

	CreatedAt time.Time
	UpdatedAt time.Time
}
