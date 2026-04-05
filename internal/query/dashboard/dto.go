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
	Total       int     `json:"total"`
	Completed   int     `json:"completed"`
	Cancelled   int     `json:"cancelled"`
	NoShow      int     `json:"no_show"`
	Scheduled   int     `json:"scheduled"`
	AttendanceRate float64 `json:"attendance_rate"` // completed / (completed + cancelled + no_show)
}

// RevenueDTO breaks down revenue by origin.
type RevenueDTO struct {
	TotalCents        int64 `json:"total_cents"`
	ServicesCents     int64 `json:"services_cents"`     // from appointment closures
	ProductsCents     int64 `json:"products_cents"`     // from paid orders (product type)
	SubscriptionsCents int64 `json:"subscriptions_cents"` // closures covered by subscription
}

// ClientsDTO shows new vs. returning clients in the period.
type ClientsDTO struct {
	Total     int `json:"total"`     // unique clients with appointments in period
	New       int `json:"new"`       // first appointment ever within period
	Returning int `json:"returning"` // had appointments before the period
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
	Period     string `json:"period"`      // day|week|month
	DateFrom   string `json:"date_from"`   // YYYY-MM-DD (local)
	DateTo     string `json:"date_to"`     // YYYY-MM-DD (local, inclusive)
	Timezone   string `json:"timezone"`

	Production ProductionDTO `json:"production"`
	Revenue    RevenueDTO    `json:"revenue"`
	Clients    ClientsDTO    `json:"clients"`

	TopServices []ServiceRankItem `json:"top_services"` // top 5 by revenue
	TopProducts []ProductRankItem `json:"top_products"`  // top 5 by revenue
}
