package financial

// PeriodType defines the aggregation window.
type PeriodType string

const (
	PeriodWeek  PeriodType = "week"
	PeriodMonth PeriodType = "month"
)

// RealizedDTO is revenue already confirmed in the period.
type RealizedDTO struct {
	TotalCents         int64 `json:"total_cents"`
	ServicesCents      int64 `json:"services_cents"`      // from appointment_closures
	ProductsCents      int64 `json:"products_cents"`      // from paid orders
	SubscriptionsCents int64 `json:"subscriptions_cents"` // closures covered by subscription plan
	ClosuresCount      int   `json:"closures_count"`
	PaidOrdersCount    int   `json:"paid_orders_count"`
}

// ExpectationItemDTO is a single future appointment contributing to expectation.
type ExpectationDTO struct {
	TotalCents        int64 `json:"total_cents"`
	ServicesCents     int64 `json:"services_cents"`     // sum of service prices for future appts
	SuggestionsCents  int64 `json:"suggestions_cents"`  // sum of suggested product prices
	AppointmentsCount int   `json:"appointments_count"` // future scheduled/awaiting_payment
}

// PresumedDTO is past appointments without a closure — barber didn't close them.
type PresumedDTO struct {
	TotalCents        int64 `json:"total_cents"`
	AppointmentsCount int   `json:"appointments_count"`
}

// LossItemDTO is a single loss event.
type LossItemDTO struct {
	Type        string `json:"type"`         // no_show|late_cancel|suggestion_not_sold|unclosed
	AmountCents int64  `json:"amount_cents"`
	Count       int    `json:"count"`
}

// LossesDTO aggregates all loss categories.
type LossesDTO struct {
	TotalCents       int64         `json:"total_cents"`
	Breakdown        []LossItemDTO `json:"breakdown"`
}

// ResponseDTO is the full financial view for the period.
type ResponseDTO struct {
	Period   string `json:"period"`    // week|month
	DateFrom string `json:"date_from"` // YYYY-MM-DD (local)
	DateTo   string `json:"date_to"`   // YYYY-MM-DD (local, inclusive)
	Timezone string `json:"timezone"`

	Realized    RealizedDTO    `json:"realized"`
	Expectation ExpectationDTO `json:"expectation"`
	Presumed    PresumedDTO    `json:"presumed"`
	Losses      LossesDTO      `json:"losses"`
}
