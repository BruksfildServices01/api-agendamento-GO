package payment

func ValidateCreation(
	appointmentID uint,
	amount float64,
) error {

	if appointmentID == 0 {
		return ErrInvalidAppointment()
	}

	if amount <= 0 {
		return ErrInvalidAmount()
	}

	return nil
}
