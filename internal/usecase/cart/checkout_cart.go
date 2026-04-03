package cart

import (
	"context"
	"errors"
	"strings"

	orderDomain "github.com/BruksfildServices01/barber-scheduler/internal/domain/order"
	ucOrder "github.com/BruksfildServices01/barber-scheduler/internal/usecase/order"
)

var (
	ErrCheckoutInvalidCartKey    = errors.New("invalid_cart_key")
	ErrCheckoutInvalidBarbershop = errors.New("invalid_barbershop_id")
	ErrCheckoutEmptyCart         = errors.New("empty_cart")
)

type CheckoutCartInput struct {
	CartKey      string
	BarbershopID uint
}

type CheckoutCart struct {
	store         CartStore
	createOrderUC *ucOrder.CreateOrder
}

func NewCheckoutCart(
	store CartStore,
	createOrderUC *ucOrder.CreateOrder,
) *CheckoutCart {
	return &CheckoutCart{
		store:         store,
		createOrderUC: createOrderUC,
	}
}

func (uc *CheckoutCart) Execute(
	ctx context.Context,
	input CheckoutCartInput,
) (*orderDomain.Order, error) {
	cartKey := strings.TrimSpace(input.CartKey)
	if cartKey == "" {
		return nil, ErrCheckoutInvalidCartKey
	}
	if input.BarbershopID == 0 {
		return nil, ErrCheckoutInvalidBarbershop
	}

	cart, err := uc.store.Get(ctx, cartKey, input.BarbershopID)
	if err != nil {
		return nil, err
	}
	if cart == nil || len(cart.Items) == 0 {
		return nil, ErrCheckoutEmptyCart
	}

	items := make([]ucOrder.CreateOrderItemInput, 0, len(cart.Items))
	for _, item := range cart.Items {
		if item.ProductID == 0 || item.Quantity < 1 {
			return nil, ErrCheckoutEmptyCart
		}

		items = append(items, ucOrder.CreateOrderItemInput{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	order, err := uc.createOrderUC.Execute(
		ctx,
		ucOrder.CreateOrderInput{
			BarbershopID: input.BarbershopID,
			ClientID:     nil,
			Items:        items,
		},
	)
	if err != nil {
		return nil, err
	}

	if err := uc.store.Clear(ctx, cartKey, input.BarbershopID); err != nil {
		return nil, err
	}

	return order, nil
}
