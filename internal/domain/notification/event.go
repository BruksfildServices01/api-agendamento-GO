package notification

import "time"

type EventType string

const (
	EventPaymentConfirmed     EventType = "payment_confirmed"
	EventAppointmentCancelled EventType = "appointment_cancelled"
	EventAppointmentReminder  EventType = "appointment_reminder"
)

type Event struct {
	Type EventType

	BarbershopName string

	ClientEmail string
	ClientName  string

	AppointmentID uint
	Title         string
	Description   string

	StartTime time.Time
	EndTime   time.Time
	Timezone  string
}
