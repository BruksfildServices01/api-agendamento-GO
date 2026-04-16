package paymentconfig

func Default(barbershopID uint) *Config {
	return &Config{
		BarbershopID:         barbershopID,
		RequirePixOnBooking:  false,
		DefaultRequirement:   PaymentNone,
		PixExpirationMinutes: 3,
		AcceptCash:           false,
		AcceptPix:            false,
		AcceptCredit:         false,
		AcceptDebit:          false,
	}
}
