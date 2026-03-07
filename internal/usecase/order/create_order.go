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
	Items        []CreateOrderItemInput `json:"items"`
}

func (uc *CreateOrder) Execute(
	ctx context.Context,
	input CreateOrderInput,
) (*orderDomain.Order, error) {

	// Validações mínimas (usecase-level)
	if input.BarbershopID == 0 {
		return nil, errors.New("invalid_barbershop_id")
	}
	if len(input.Items) == 0 {
		return nil, errors.New("empty_items")
	}

	var createdOrder *orderDomain.Order

	err := uc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tx = tx.WithContext(ctx)

		// ✅ amarra repos na MESMA TX
		orderRepoTx := uc.orderRepository.WithTx(tx)
		productRepoTx := uc.productRepository.WithTx(tx)

		order := orderDomain.New(
			input.BarbershopID,
			orderDomain.OrderTypeProduct,
		)

		for _, item := range input.Items {
			// leitura dentro da tx
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

			// decremento dentro da tx (repo já é tx-aware)
			if err := productRepoTx.DecreaseStock(
				ctx,
				input.BarbershopID,
				product.ID,
				item.Quantity,
			); err != nil {
				return err
			}

			// adiciona item no agregado do pedido
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

		// 🔥 TESTE TEMPORÁRIO: força rollback após mexer em estoque e montar o pedido
		// Remova esta linha depois de validar no banco que stock NÃO mudou e order NÃO foi criado.
		// return errors.New("force_rollback_test")

		// ✅ cria order+items dentro da MESMA tx (repo NÃO abre tx interna)
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
