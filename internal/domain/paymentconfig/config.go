package paymentconfig

type Config struct {
	BarbershopID uint

	RequirePixOnBooking  bool
	PixExpirationMinutes int
	DefaultRequirement   PaymentRequirement

	MPAccessToken string
	MPPublicKey   string
}
