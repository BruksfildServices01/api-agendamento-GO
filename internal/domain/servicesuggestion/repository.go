package servicesuggestion

import "context"

type Repository interface {
	SetSuggestion(
		ctx context.Context,
		barbershopID uint,
		serviceID uint,
		productID uint,
	) error

	GetSuggestionByServiceID(
		ctx context.Context,
		barbershopID uint,
		serviceID uint,
	) (*ServiceSuggestion, error)

	RemoveSuggestion(
		ctx context.Context,
		barbershopID uint,
		serviceID uint,
	) error

	GetPublicSuggestionByServiceID(
		ctx context.Context,
		barbershopID uint,
		serviceID uint,
	) (*ServiceSuggestion, error)
}
