package bookingpolicy

import "context"

type Repository interface {
	GetByCategory(
		ctx context.Context,
		barbershopID uint,
		category Category,
	) (*BookingPolicy, error)

	Upsert(
		ctx context.Context,
		policy *BookingPolicy,
	) error
}
