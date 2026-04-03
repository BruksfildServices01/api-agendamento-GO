package dto

import "time"

type PublicOrchestratedCheckoutAppointmentDTO struct {
	ID                 uint      `json:"id"`
	Status             string    `json:"status"`
	StartTime          time.Time `json:"start_time"`
	EndTime            time.Time `json:"end_time"`
	ServiceID          uint      `json:"service_id"`
	ServiceName        string    `json:"service_name"`
	ServiceAmountCents int64     `json:"service_amount_cents"`
}

type PublicOrchestratedCheckoutOrderDTO struct {
	ID         uint   `json:"id"`
	Status     string `json:"status"`
	TotalCents int64  `json:"total_cents"`
	ItemsCount int    `json:"items_count"`
}

type PublicOrchestratedCheckoutSummaryDTO struct {
	ServiceAmountCents  int64 `json:"service_amount_cents"`
	ProductsAmountCents int64 `json:"products_amount_cents"`
	TotalAmountCents    int64 `json:"total_amount_cents"`
}

type PublicOrchestratedCheckoutNextStepDTO struct {
	Action   string `json:"action"`
	Method   string `json:"method,omitempty"`
	Guidance string `json:"guidance,omitempty"`
}

type PublicOrchestratedCheckoutURLsDTO struct {
	AppointmentPixURL string `json:"appointment_pix_url,omitempty"`
	OrderPixURL       string `json:"order_pix_url,omitempty"`
}

type PublicOrchestratedCheckoutPaymentsDTO struct {
	AppointmentPaymentRequired bool `json:"appointment_payment_required"`
	OrderPaymentRequired       bool `json:"order_payment_required"`
	MultiplePaymentsRequired   bool `json:"multiple_payments_required"`
}

type PublicOrchestratedCheckoutResponseDTO struct {
	Appointment *PublicOrchestratedCheckoutAppointmentDTO `json:"appointment,omitempty"`
	Order       *PublicOrchestratedCheckoutOrderDTO       `json:"order,omitempty"`
	Summary     PublicOrchestratedCheckoutSummaryDTO      `json:"summary"`
	Payments    PublicOrchestratedCheckoutPaymentsDTO     `json:"payments"`
	NextStep    PublicOrchestratedCheckoutNextStepDTO     `json:"next_step"`
	NextURLs    PublicOrchestratedCheckoutURLsDTO         `json:"next_urls"`
	Warning     string                                    `json:"warning,omitempty"`
}
