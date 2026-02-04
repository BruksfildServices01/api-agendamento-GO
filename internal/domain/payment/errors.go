package payment

import "github.com/BruksfildServices01/barber-scheduler/internal/httperr"

func ErrInvalidAmount() error {
	return httperr.ErrBusiness("invalid_amount")
}

func ErrInvalidAppointment() error {
	return httperr.ErrBusiness("invalid_appointment")
}

func ErrInvalidState() error {
	return httperr.ErrBusiness("invalid_state")
}
