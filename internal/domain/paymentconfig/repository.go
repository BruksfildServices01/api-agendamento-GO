package paymentconfig

import "context"

type Repository interface {
	GetByBarbershopID(
		ctx context.Context,
		barbershopID uint,
	) (*Config, error)
}
