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

func (uc *ListPaymentsForBarbershop) Execute(
	ctx context.Context,
	input ListPaymentsInput,
) ([]models.Payment, error) {

	if input.BarbershopID == 0 {
		return nil, errors.New("invalid barbershop id")
	}

	filter := domain.PaymentListFilter{
		Status:    input.Status,
		StartDate: input.StartDate,
		EndDate:   input.EndDate,
	}

	return uc.repo.ListForBarbershop(
		ctx,
		input.BarbershopID,
		filter,
	)
}
