package bookingpolicy

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/bookingpolicy"
)

type ResolveClientBookingPolicy struct {
	repo domain.Repository
}

func NewResolveClientBookingPolicy(
	repo domain.Repository,
) *ResolveClientBookingPolicy {
	return &ResolveClientBookingPolicy{repo: repo}
}

func (uc *ResolveClientBookingPolicy) Execute(
	ctx context.Context,
	barbershopID uint,
	category domain.Category,
) (*domain.BookingPolicy, error) {

	policy, err := uc.repo.GetByCategory(ctx, barbershopID, category)
	if err != nil {
		return nil, err
	}

	if policy != nil {
		return policy, nil
	}

	return &domain.BookingPolicy{
		BarbershopID:        barbershopID,
		Category:            category,
		RequirePrePayment:   false,
		AllowPayLater:       true,
		AllowManualOverride: true,
	}, nil
}
