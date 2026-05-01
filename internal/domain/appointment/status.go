package appointment

import "github.com/BruksfildServices01/barber-scheduler/internal/apperr"

type Status string

const (
	StatusScheduled       Status = "scheduled"
	StatusAwaitingPayment Status = "awaiting_payment"
	StatusCancelled       Status = "cancelled"
	StatusCompleted       Status = "completed"
	StatusNoShow          Status = "no_show"
)

// --------------------------------------
// Regras de transição
// --------------------------------------

func CanCancel(current Status) error {
	switch current {
	case StatusScheduled, StatusAwaitingPayment:
		return nil
	default:
		return apperr.ErrBusiness("invalid_state")
	}
}

func CanComplete(current Status) error {
	switch current {
	case StatusScheduled, StatusAwaitingPayment:
		return nil
	default:
		return apperr.ErrBusiness("invalid_state")
	}
}

func CanMarkNoShow(current Status) error {
	switch current {
	case StatusScheduled, StatusAwaitingPayment:
		return nil
	default:
		return apperr.ErrBusiness("invalid_state")
	}
}

// --------------------------------------
// Estados iniciais
// --------------------------------------

func InitialStatus() Status {
	return StatusScheduled
}
