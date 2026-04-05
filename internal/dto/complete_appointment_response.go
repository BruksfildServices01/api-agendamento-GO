package dto

import "github.com/BruksfildServices01/barber-scheduler/internal/models"

type CompleteAppointmentSubscriptionDTO struct {
	ConsumeStatus string `json:"consume_status"`
	PlanID        *uint  `json:"plan_id,omitempty"`
}

type CompleteAppointmentOperationalDTO struct {
	// Serviço agendado originalmente
	ServiceID   *uint  `json:"service_id,omitempty"`
	ServiceName string `json:"service_name,omitempty"`

	// Serviço realizado (pode diferir do agendado)
	ActualServiceID   *uint  `json:"actual_service_id,omitempty"`
	ActualServiceName string `json:"actual_service_name,omitempty"`

	ReferenceAmountCents int64  `json:"reference_amount_cents"`
	FinalAmountCents     *int64 `json:"final_amount_cents,omitempty"`

	PaymentMethod     string `json:"payment_method,omitempty"`
	SuggestionRemoved bool   `json:"suggestion_removed"`

	// Venda adicional gerada no fechamento
	AdditionalOrderID *uint `json:"additional_order_id,omitempty"`

	OperationalNote           string `json:"operational_note,omitempty"`
	SubscriptionConsumeStatus string `json:"subscription_consume_status,omitempty"`
	SubscriptionCovered       bool   `json:"subscription_covered"`
	RequiresNormalCharging    bool   `json:"requires_normal_charging"`
	ConfirmNormalCharging     bool   `json:"confirm_normal_charging"`
}

type CompleteAppointmentResponse struct {
	Appointment  *models.Appointment                 `json:"appointment"`
	Subscription *CompleteAppointmentSubscriptionDTO `json:"subscription,omitempty"`
	Operational  CompleteAppointmentOperationalDTO   `json:"operational"`
}
