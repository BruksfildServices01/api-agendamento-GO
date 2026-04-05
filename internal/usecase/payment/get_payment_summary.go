package payment

import (
	"context"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
)

type GetPaymentSummary struct {
	repo domain.Repository
}

func NewGetPaymentSummary(
	repo domain.Repository,
) *GetPaymentSummary {
	return &GetPaymentSummary{
		repo: repo,
	}
}

func (uc *GetPaymentSummary) Execute(
	ctx context.Context,
	barbershopID uint,
	from *time.Time,
	to *time.Time,
) (*domain.PaymentSummary, error) {

	return uc.repo.GetSummaryForBarbershop(
		ctx,
		barbershopID,
		from,
		to,
	)
}
