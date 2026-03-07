package payment

import (
	"fmt"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
)

func ErrInvalidAmount() error {
	return httperr.ErrBusiness("invalid_amount")
}

func ErrInvalidAppointment() error {
	return httperr.ErrBusiness("invalid_appointment")
}

func ErrInvalidState() error {
	return httperr.ErrBusiness("invalid_state")
}

func ErrInvalidPaymentTransition(from Status, to Status) error {
	return fmt.Errorf("invalid payment transition: %s -> %s", from, to)
}

func ErrInvalidTarget() error {
	return httperr.ErrBusiness("invalid_payment_target")
}
