package crm

import "time"

// IdentityDTO carries the client's basic contact information.
type IdentityDTO struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

// CategoryDTO carries the resolved behavioral category and its source.
type CategoryDTO struct {
	Value  string `json:"value"`  // new|regular|trusted|at_risk
	Source string `json:"source"` // auto|manual
}

// MetricsDTO carries the raw counters used for classification.
type MetricsDTO struct {
	TotalAppointments       int        `json:"total_appointments"`
	Completed               int        `json:"completed"`
	Cancelled               int        `json:"cancelled"`
	LateCancelled           int        `json:"late_cancelled"`
	NoShow                  int        `json:"no_show"`
	Rescheduled             int        `json:"rescheduled"`
	LateRescheduled         int        `json:"late_rescheduled"`
	TotalSpentCents         int64      `json:"total_spent_cents"`
	AttendanceRate          float64    `json:"attendance_rate"`
	FirstAppointmentAt      *time.Time `json:"first_appointment_at,omitempty"`
	LastAppointmentAt       *time.Time `json:"last_appointment_at,omitempty"`
	LastCompletedAt         *time.Time `json:"last_completed_at,omitempty"`
}

// SubscriptionDTO carries the active plan context (nil when no active plan).
type SubscriptionDTO struct {
	PlanID       uint      `json:"plan_id"`
	PlanName     string    `json:"plan_name"`
	CutsUsed     int       `json:"cuts_used"`
	CutsIncluded int       `json:"cuts_included"`
	ValidUntil   time.Time `json:"valid_until"`
}

// FlagsDTO are pre-computed boolean signals for fast operational decisions.
type FlagsDTO struct {
	Premium   bool `json:"premium"`   // has active subscription
	Reliable  bool `json:"reliable"`  // attendance_rate >= 0.9, total >= 5
	Attention bool `json:"attention"` // attendance_rate < 0.7
	AtRisk    bool `json:"at_risk"`   // category == at_risk
}

// PolicyDTO is the booking policy applied to this client.
type PolicyDTO struct {
	RequiresPaymentUpfront bool   `json:"requires_payment_upfront"`
	Reason                 string `json:"reason,omitempty"`
}

// ResponseDTO is the full CRM card for a single client.
type ResponseDTO struct {
	Identity     IdentityDTO      `json:"identity"`
	Category     CategoryDTO      `json:"category"`
	Metrics      MetricsDTO       `json:"metrics"`
	Flags        FlagsDTO         `json:"flags"`
	Subscription *SubscriptionDTO `json:"subscription,omitempty"`
	Policy       PolicyDTO        `json:"policy"`
}
