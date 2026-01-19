package appointment

import "github.com/BruksfildServices01/barber-scheduler/internal/httperr"

// ===============================
// Appointment Status
// ===============================

type Status string

const (
	StatusScheduled Status = "scheduled"
	StatusCancelled Status = "cancelled"
	StatusCompleted Status = "completed"
)

// ===============================
// Validations
// ===============================

// CanCancel define se um agendamento pode ser cancelado
func CanCancel(current Status) error {
	if current != StatusScheduled {
		return httperr.ErrBusiness("invalid_state")
	}
	return nil
}

// CanComplete define se um agendamento pode ser concluído
func CanComplete(current Status) error {
	if current != StatusScheduled {
		return httperr.ErrBusiness("invalid_state")
	}
	return nil
}

// CanCreate valida status inicial
func InitialStatus() Status {
	return StatusScheduled
}
