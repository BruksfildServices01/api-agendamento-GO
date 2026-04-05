package payment

func ValidateCreation(
	appointmentID *uint,
	orderID *uint,
	amountCents int64,
) error {

	if appointmentID == nil && orderID == nil {
		return ErrInvalidTarget()
	}

	if appointmentID != nil && orderID != nil {
		return ErrInvalidTarget()
	}

	if amountCents <= 0 {
		return ErrInvalidAmount()
	}

	return nil
}
