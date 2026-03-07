package paymentconfig

import "context"

type Repository interface {
	GetByBarbershopID(
		ctx context.Context,
		barbershopID uint,
	) (*Config, error)

	ListCategoryPolicies(
		ctx context.Context,
		barbershopID uint,
	) ([]CategoryPaymentPolicy, error)

	UpsertCategoryPolicy(
		ctx context.Context,
		barbershopID uint,
		policy CategoryPaymentPolicy,
	) error

	UpsertConfig(
		ctx context.Context,
		cfg *Config,
	) error

	DeleteCategoryPolicies(
		ctx context.Context,
		barbershopID uint,
	) error
}
