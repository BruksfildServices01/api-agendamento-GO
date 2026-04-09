package dashboard

// PeriodType defines the aggregation window.
type PeriodType string

const (
	PeriodDay   PeriodType = "day"
	PeriodWeek  PeriodType = "week"
	PeriodMonth PeriodType = "month"
)

// ProductionDTO aggregates appointment outcomes for the period.
type ProductionDTO struct {
	Total          int     `json:"total"`
	Completed      int     `json:"completed"`
	Cancelled      int     `json:"cancelled"`
	NoShow         int     `json:"no_show"`
	Scheduled      int     `json:"scheduled"`
	AttendanceRate float64 `json:"attendance_rate"` // completed / (completed + cancelled + no_show)

	// Suggestion conversion
	SuggestionTotal   int `json:"suggestion_total"`   // closures that had an active suggestion
	SuggestionKept    int `json:"suggestion_kept"`    // suggestion was kept (sold)
	SuggestionRemoved int `json:"suggestion_removed"` // suggestion was removed (not sold)
}

// RevenueDTO breaks down revenue by origin.
type RevenueDTO struct {
	TotalCents              int64 `json:"total_cents"`
	ServicesCents           int64 `json:"services_cents"`
	ProductsCents           int64 `json:"products_cents"`
	ProductsSuggestionCents int64 `json:"products_suggestion_cents"`
	ProductsStandaloneCents int64 `json:"products_standalone_cents"`
	SubscriptionsCents      int64 `json:"subscriptions_cents"`
	AvgTicketCents          int64 `json:"avg_ticket_cents"` // avg per completed appointment
}

// ClientsDTO shows new vs. returning clients in the period.
type ClientsDTO struct {
	Total                int `json:"total"`                  // unique clients with appointments in period
	New                  int `json:"new"`                    // first appointment ever within period
	Returning            int `json:"returning"`              // had appointments before the period
	WithActiveSubscription int `json:"with_active_subscription"` // clients with active subscription
}

// ServiceRankItem is a single entry in the service ranking.
type ServiceRankItem struct {
	ServiceID    uint   `json:"service_id"`
	ServiceName  string `json:"service_name"`
	Count        int    `json:"count"`
	RevenueCents int64  `json:"revenue_cents"`
}

// ProductRankItem is a single entry in the product ranking.
type ProductRankItem struct {
	ProductID    uint   `json:"product_id"`
	ProductName  string `json:"product_name"`
	Quantity     int    `json:"quantity"`
	RevenueCents int64  `json:"revenue_cents"`
}

// ResponseDTO is the full dashboard payload for the period.
type ResponseDTO struct {
	Period   string `json:"period"`
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
	Timezone string `json:"timezone"`

	Production  ProductionDTO      `json:"production"`
	Revenue     RevenueDTO         `json:"revenue"`
	Clients     ClientsDTO         `json:"clients"`
	TopServices []ServiceRankItem  `json:"top_services"`
	TopProducts []ProductRankItem  `json:"top_products"`
}
