package payment

import (
	"context"
	"errors"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type ListPaymentsInput struct {
	BarbershopID uint

	Status    *string
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Offset    int
}

type ListPaymentsForBarbershop struct {
	repo domain.Repository
}

func NewListPaymentsForBarbershop(
	repo domain.Repository,
) *ListPaymentsForBarbershop {
	return &ListPaymentsForBarbershop{
		repo: repo,
	}
}

type ListPaymentsResult struct {
	Payments []models.Payment
	Total    int64
}

func (uc *ListPaymentsForBarbershop) Execute(
	ctx context.Context,
	input ListPaymentsInput,
) (*ListPaymentsResult, error) {

	if input.BarbershopID == 0 {
		return nil, errors.New("invalid barbershop id")
	}

	filter := domain.PaymentListFilter{
		Status:    input.Status,
		StartDate: input.StartDate,
		EndDate:   input.EndDate,
		Limit:     input.Limit,
		Offset:    input.Offset,
	}

	payments, err := uc.repo.ListForBarbershop(ctx, input.BarbershopID, filter)
	if err != nil {
		return nil, err
	}

	total, err := uc.repo.CountForBarbershop(ctx, input.BarbershopID, filter)
	if err != nil {
		return nil, err
	}

	return &ListPaymentsResult{Payments: payments, Total: total}, nil
}
