package order

import (
	"context"
	"errors"

	orderDomain "github.com/BruksfildServices01/barber-scheduler/internal/domain/order"
	infraRepo "github.com/BruksfildServices01/barber-scheduler/internal/infra/repository"
)

type ListOrders struct {
	orderRepository *infraRepo.OrderGormRepository
}

func NewListOrders(
	orderRepo *infraRepo.OrderGormRepository,
) *ListOrders {
	return &ListOrders{
		orderRepository: orderRepo,
	}
}

func (uc *ListOrders) Execute(
	ctx context.Context,
	barbershopID uint,
) ([]orderDomain.Order, error) {
	if barbershopID == 0 {
		return nil, errors.New("invalid_barbershop_id")
	}

	return uc.orderRepository.ListByBarbershop(ctx, barbershopID)
}
