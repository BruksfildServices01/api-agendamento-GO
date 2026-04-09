package financial

// PeriodType defines the aggregation window.
type PeriodType string

const (
	PeriodWeek  PeriodType = "week"
	PeriodMonth PeriodType = "month"
)

// RealizedDTO is revenue already confirmed in the period.
type RealizedDTO struct {
	TotalCents                  int64 `json:"total_cents"`
	ServicesCents               int64 `json:"services_cents"`               // from appointment_closures (net of subscription)
	ProductsCents               int64 `json:"products_cents"`               // total from paid orders
	ProductsSuggestionCents     int64 `json:"products_suggestion_cents"`    // orders linked via closure.additional_order_id
	ProductsStandaloneCents     int64 `json:"products_standalone_cents"`    // orders not linked to any closure
	SubscriptionsCents          int64 `json:"subscriptions_cents"`          // closures covered by subscription plan
	ClosuresCount               int   `json:"closures_count"`
	PaidOrdersCount             int   `json:"paid_orders_count"`
}

// ExpectationDTO aggregates future scheduled appointments.
type ExpectationDTO struct {
	TotalCents        int64 `json:"total_cents"`
	ServicesCents     int64 `json:"services_cents"`
	SuggestionsCents  int64 `json:"suggestions_cents"`
	AppointmentsCount int   `json:"appointments_count"`
}

// PresumedDTO is past appointments without a closure.
type PresumedDTO struct {
	TotalCents        int64 `json:"total_cents"`
	AppointmentsCount int   `json:"appointments_count"`
}

// LossItemDTO is a single loss event.
type LossItemDTO struct {
	Type        string `json:"type"`
	AmountCents int64  `json:"amount_cents"`
	Count       int    `json:"count"`
}

// LossesDTO aggregates all loss categories.
type LossesDTO struct {
	TotalCents int64         `json:"total_cents"`
	Breakdown  []LossItemDTO `json:"breakdown"`
}

// TopItemDTO represents a ranked service or product.
type TopItemDTO struct {
	Name         string `json:"name"`
	Count        int    `json:"count"`
	RevenueCents int64  `json:"revenue_cents"`
}

// ResponseDTO is the full financial view for the period.
type ResponseDTO struct {
	Period   string `json:"period"`
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
	Timezone string `json:"timezone"`

	Realized    RealizedDTO    `json:"realized"`
	Expectation ExpectationDTO `json:"expectation"`
	Presumed    PresumedDTO    `json:"presumed"`
	Losses      LossesDTO      `json:"losses"`

	TopServices []TopItemDTO `json:"top_services"`
	TopProducts []TopItemDTO `json:"top_products"`
}
