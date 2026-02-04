package paymentconfig

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/paymentconfig"
)

type BookingPaymentPolicy struct {
	RequirePix           bool
	PixExpirationMinutes int
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

	return &BookingPaymentPolicy{
		RequirePix:           cfg.RequirePixOnBooking,
		PixExpirationMinutes: cfg.PixExpirationMinutes,
	}, nil
}
