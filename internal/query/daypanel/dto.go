package daypanel

import "time"

// ClientDTO carries the client identity and behavioral classification.
type ClientDTO struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Category string `json:"category"` // new|regular|trusted|at_risk
}

// ServiceDTO carries the service being performed.
type ServiceDTO struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	DurationMin int    `json:"duration_min"`
	PriceCents  int64  `json:"price_cents"`
}

// PaymentDTO carries the PIX payment status for the appointment.
// Status is "none" when no payment record exists.
type PaymentDTO struct {
	Status      string     `json:"status"` // none|pending|paid|expired
	AmountCents int64      `json:"amount_cents"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
}

// SuggestionDTO is the product suggested for the service being performed.
// Represents "item previsto" — the barber should have this product ready.
type SuggestionDTO struct {
	ProductID   uint   `json:"product_id"`
	ProductName string `json:"product_name"`
	PriceCents  int64  `json:"price_cents"`
}

// PrePaidOrderDTO is an order the client already paid (via public checkout)
// on the same day as the appointment.
// Represents "item pago antecipadamente" — products already charged.
type PrePaidOrderDTO struct {
	OrderID    uint       `json:"order_id"`
	TotalCents int64      `json:"total_cents"`
	ItemsCount int        `json:"items_count"`
	PaidAt     *time.Time `json:"paid_at,omitempty"`
}

// SubscriptionDTO carries the client's active subscription context.
// ServiceCovered signals whether this appointment's service is included in the plan.
type SubscriptionDTO struct {
	PlanID         uint      `json:"plan_id"`
	PlanName       string    `json:"plan_name"`
	CutsUsed       int       `json:"cuts_used"`
	CutsIncluded   int       `json:"cuts_included"`
	ValidUntil     time.Time `json:"valid_until"`
	ServiceCovered bool      `json:"service_covered"`
}

// FlagsDTO are pre-computed operational alerts the barber acts on without opening other screens.
type FlagsDTO struct {
	AwaitingPayment bool `json:"awaiting_payment"`  // appointment needs PIX to confirm
	HasPrePaidItems bool `json:"has_pre_paid_items"` // products already charged via store
	HasSuggestion   bool `json:"has_suggestion"`     // recommended product for this service
	IsAtRisk        bool `json:"is_at_risk"`         // client has at_risk behavior
	HasSubscription bool `json:"has_subscription"`   // client has active plan
}

// CardDTO is the complete operational card for a single appointment.
type CardDTO struct {
	AppointmentID uint      `json:"appointment_id"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	Status        string    `json:"status"`
	CreatedBy     string    `json:"created_by"`
	Notes         string    `json:"notes,omitempty"`

	Client       ClientDTO        `json:"client"`
	Service      ServiceDTO       `json:"service"`
	Payment      PaymentDTO       `json:"payment"`
	Suggestion   *SuggestionDTO   `json:"suggestion,omitempty"`
	PrePaidOrder *PrePaidOrderDTO `json:"pre_paid_order,omitempty"`
	Subscription *SubscriptionDTO `json:"subscription,omitempty"`
	Flags        FlagsDTO         `json:"flags"`
}

// SummaryDTO aggregates counts for the day.
type SummaryDTO struct {
	Total           int `json:"total"`
	Scheduled       int `json:"scheduled"`
	AwaitingPayment int `json:"awaiting_payment"`
	Completed       int `json:"completed"`
	Cancelled       int `json:"cancelled"`
	NoShow          int `json:"no_show"`
}

// ResponseDTO is the full day panel payload returned to the client.
type ResponseDTO struct {
	Date     string     `json:"date"`
	Timezone string     `json:"timezone"`
	Cards    []CardDTO  `json:"cards"`
	Summary  SummaryDTO `json:"summary"`
}
