package impact

type PeriodType string

const (
	PeriodWeek  PeriodType = "week"
	PeriodMonth PeriodType = "month"
)

type Input struct {
	BarbershopID uint
	Period       PeriodType
}

// RevenueDTO — faturamento atual vs período anterior.
type RevenueDTO struct {
	CurrentCents       int64   `json:"current_cents"`
	PreviousCents      int64   `json:"previous_cents"`
	GrowthPercent      float64 `json:"growth_percent"`
	TicketAverageCents int64   `json:"ticket_average_cents"`
}

// TrendPointDTO — ponto de evolução temporal dentro do período.
type TrendPointDTO struct {
	Label        string `json:"label"`
	Count        int    `json:"count"`
	RevenueCents int64  `json:"revenue_cents"`
}

// GrowthDTO — crescimento de clientes e atendimentos.
type GrowthDTO struct {
	NewClientsCount       int             `json:"new_clients_count"`
	ReturningClientsCount int             `json:"returning_clients_count"`
	TotalActiveClients    int             `json:"total_active_clients"`
	AppointmentsCount     int             `json:"appointments_count"`
	Trend                 []TrendPointDTO `json:"trend"`
}

// RetentionDTO — retenção e risco de churn.
type RetentionDTO struct {
	ReturnRatePercent float64 `json:"return_rate_percent"`
	AtRiskCount       int     `json:"at_risk_count"`
	TrustedCount      int     `json:"trusted_count"`
	InactiveCount     int     `json:"inactive_count"`
}

// LossesDTO — perdas do período.
type LossesDTO struct {
	TotalCents        int64 `json:"total_cents"`
	NoShowCents       int64 `json:"no_show_cents"`
	NoShowCount       int   `json:"no_show_count"`
	CancellationCents int64 `json:"cancellation_cents"`
	CancellationCount int   `json:"cancellation_count"`
}

// UsageDTO — uso do sistema.
type UsageDTO struct {
	TotalAppointments     int     `json:"total_appointments"`
	CompletedCount        int     `json:"completed_count"`
	AttendanceRatePercent float64 `json:"attendance_rate_percent"`
	ClosuresCount         int     `json:"closures_count"`
	ClosureRatePercent    float64 `json:"closure_rate_percent"`
	AdjustmentsCount      int     `json:"adjustments_count"`
}

// IndirectDTO — ganhos indiretos.
type IndirectDTO struct {
	AdditionalSalesCents     int64   `json:"additional_sales_cents"`
	AdditionalSalesCount     int     `json:"additional_sales_count"`
	ActiveSubscriptionsCount int     `json:"active_subscriptions_count"`
	SuggestionConversionRate float64 `json:"suggestion_conversion_rate_percent"`
	UpsellCapturedCents      int64   `json:"upsell_captured_cents"`
}

// ROIDTO — valor gerado pelo sistema.
type ROIDTO struct {
	ValueGeneratedCents    int64  `json:"value_generated_cents"`
	LossesMitigatedNote    string `json:"losses_mitigated_note"`
	SubscriptionValueCents int64  `json:"subscription_value_cents"`
	JustificationNote      string `json:"justification_note"`
}

// ResponseDTO — relatório completo.
type ResponseDTO struct {
	Period   string `json:"period"`
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
	Timezone string `json:"timezone"`

	Revenue   RevenueDTO   `json:"revenue"`
	Growth    GrowthDTO    `json:"growth"`
	Retention RetentionDTO `json:"retention"`
	Losses    LossesDTO    `json:"losses"`
	Usage     UsageDTO     `json:"usage"`
	Indirect  IndirectDTO  `json:"indirect_gains"`
	ROI       ROIDTO       `json:"roi"`
}
