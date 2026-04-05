package paymentconfig

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/paymentconfig"
)

type BookingPaymentPolicy struct {
	PixExpirationMinutes int

	// Regra global default (vem do DB)
	DefaultRequirement domain.PaymentRequirement

	// Regras específicas por categoria comportamental (vem do DB)
	CategoryPolicies domain.CategoryPaymentPolicies
}

type ResolveBookingPaymentPolicy struct {
	repo domain.Repository
}

func NewResolveBookingPaymentPolicy(
	repo domain.Repository,
) *ResolveBookingPaymentPolicy {
	return &ResolveBookingPaymentPolicy{repo: repo}
}

func (uc *ResolveBookingPaymentPolicy) Execute(
	ctx context.Context,
	barbershopID uint,
) (*BookingPaymentPolicy, error) {
	cfg, err := uc.repo.GetByBarbershopID(ctx, barbershopID)
	if err != nil {
		return nil, err
	}

	categories, err := uc.repo.ListCategoryPolicies(ctx, barbershopID)
	if err != nil {
		return nil, err
	}

	categoryPolicies := domain.CategoryPaymentPolicies(categories)

	return &BookingPaymentPolicy{
		PixExpirationMinutes: cfg.PixExpirationMinutes,
		DefaultRequirement:   cfg.DefaultRequirement,
		CategoryPolicies:     categoryPolicies,
	}, nil
}
