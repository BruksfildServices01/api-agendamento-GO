package paymentconfig

func Default(barbershopID uint) *Config {
	return &Config{
		BarbershopID:         barbershopID,
		RequirePixOnBooking:  false,
		DefaultRequirement:   PaymentOptional,
		PixExpirationMinutes: 15,
	}
}
