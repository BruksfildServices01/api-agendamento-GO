package product

import (
	"context"
	"errors"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/product"
)

type ListProducts struct {
	repo domain.Repository
}

func NewListProducts(repo domain.Repository) *ListProducts {
	return &ListProducts{repo: repo}
}

func (uc *ListProducts) Execute(
	ctx context.Context,
	barbershopID uint,
) ([]*domain.Product, error) {
	if barbershopID == 0 {
		return nil, errors.New("invalid_barbershop_id")
	}

	return uc.repo.ListByBarbershop(ctx, barbershopID)
}
