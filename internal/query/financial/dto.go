package financial

// PeriodType defines the aggregation window.
type PeriodType string

const (
	PeriodWeek  PeriodType = "week"
	PeriodMonth PeriodType = "month"
)

// RealizedDTO is revenue already confirmed in the period.
type RealizedDTO struct {
	// Dinheiro recebido: serviços pagos avulsos + produtos + mensalidades de assinatura.
	TotalCents                      int64 `json:"total_cents"`
	// Serviços pagos sem cobertura de assinatura (net).
	ServicesCents                   int64 `json:"services_cents"`
	// Total de produtos pagos.
	ProductsCents                   int64 `json:"products_cents"`
	ProductsSuggestionCents         int64 `json:"products_suggestion_cents"`
	ProductsStandaloneCents         int64 `json:"products_standalone_cents"`
	// Mensalidades de assinatura pagas no período (payments.subscription_id IS NOT NULL AND status='paid').
	SubscriptionPaymentRevenueCents int64 `json:"subscription_payment_revenue_cents"`
	// Produção operacional coberta por assinatura — valor dos atendimentos cobertos.
	// Informativo: NÃO representa dinheiro recebido no período, apenas produção realizada via plano.
	SubscriptionsCents              int64 `json:"subscriptions_cents"`
	ClosuresCount                   int   `json:"closures_count"`
	PaidOrdersCount                 int   `json:"paid_orders_count"`
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
