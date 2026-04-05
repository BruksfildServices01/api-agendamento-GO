package order

import (
	"context"
	"errors"

	orderDomain "github.com/BruksfildServices01/barber-scheduler/internal/domain/order"
	infraRepo "github.com/BruksfildServices01/barber-scheduler/internal/infra/repository"
)

type GetOrder struct {
	orderRepository *infraRepo.OrderGormRepository
}

func NewGetOrder(
	orderRepo *infraRepo.OrderGormRepository,
) *GetOrder {
	return &GetOrder{
		orderRepository: orderRepo,
	}
}

func (uc *GetOrder) Execute(
	ctx context.Context,
	barbershopID uint,
	orderID uint,
) (*orderDomain.Order, error) {
	if barbershopID == 0 {
		return nil, errors.New("invalid_barbershop_id")
	}

	if orderID == 0 {
		return nil, errors.New("invalid_order_id")
	}

	return uc.orderRepository.GetByID(ctx, barbershopID, orderID)
}
