package order

import (
	"context"
	"errors"

	"gorm.io/gorm"

	orderDomain "github.com/BruksfildServices01/barber-scheduler/internal/domain/order"
	productDomain "github.com/BruksfildServices01/barber-scheduler/internal/domain/product"
	infraRepo "github.com/BruksfildServices01/barber-scheduler/internal/infra/repository"
)

type CreateOrder struct {
	db                *gorm.DB
	orderRepository   *infraRepo.OrderGormRepository
	productRepository *infraRepo.ProductGormRepository
}

func NewCreateOrder(
	db *gorm.DB,
	orderRepo *infraRepo.OrderGormRepository,
	productRepo *infraRepo.ProductGormRepository,
) *CreateOrder {
	return &CreateOrder{
		db:                db,
		orderRepository:   orderRepo,
		productRepository: productRepo,
	}
}

type CreateOrderItemInput struct {
	ProductID uint `json:"product_id"`
	Quantity  int  `json:"quantity"`
}

type CreateOrderInput struct {
	BarbershopID uint                   `json:"barbershop_id"`
	ClientID     *uint                  `json:"client_id,omitempty"`
	Items        []CreateOrderItemInput `json:"items"`
}

func (uc *CreateOrder) Execute(
	ctx context.Context,
	input CreateOrderInput,
) (*orderDomain.Order, error) {
	if input.BarbershopID == 0 {
		return nil, errors.New("invalid_barbershop_id")
	}
	if len(input.Items) == 0 {
		return nil, errors.New("empty_items")
	}

	var createdOrder *orderDomain.Order

	err := uc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tx = tx.WithContext(ctx)

		orderRepoTx := uc.orderRepository.WithTx(tx)
		productRepoTx := uc.productRepository.WithTx(tx)

		order := orderDomain.New(
			input.BarbershopID,
			input.ClientID,
		)

		for _, item := range input.Items {
			product, err := productRepoTx.GetByID(
				ctx,
				input.BarbershopID,
				item.ProductID,
			)
			if err != nil {
				return err
			}
			if product == nil {
				return productDomain.ErrProductNotFound
			}

			if err := order.AddItem(
				product.ID,
				product.Name,
				item.Quantity,
				product.Price,
			); err != nil {
				return err
			}
		}

		if err := order.Validate(); err != nil {
			return err
		}

		if err := orderRepoTx.Create(ctx, order); err != nil {
			return err
		}

		createdOrder = order
		return nil
	})

	if err != nil {
		return nil, err
	}

	return createdOrder, nil
}
