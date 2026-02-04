package appointment

import "github.com/BruksfildServices01/barber-scheduler/internal/httperr"

// ===============================
// Appointment Status
// ===============================

type Status string

const (
	StatusScheduled       Status = "scheduled"
	StatusAwaitingPayment Status = "awaiting_payment"
	StatusCancelled       Status = "cancelled"
	StatusCompleted       Status = "completed"
)

// ===============================
// Validations
// ===============================

func CanCancel(current Status) error {
	if current != StatusScheduled {
		return httperr.ErrBusiness("invalid_state")
	}
	return nil
}

func CanComplete(current Status) error {
	if current != StatusScheduled {
		return httperr.ErrBusiness("invalid_state")
	}
	return nil
}

func InitialStatus() Status {
	return StatusScheduled
}
