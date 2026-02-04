package paymentconfig

func Default(barbershopID uint) *Config {
	return &Config{
		BarbershopID:         barbershopID,
		RequirePixOnBooking:  false,
		PixExpirationMinutes: 15,
	}
}
