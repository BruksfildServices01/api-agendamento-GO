package cart

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

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
	db            *gorm.DB
}

func NewCheckoutCart(
	db *gorm.DB,
	store CartStore,
	createOrderUC *ucOrder.CreateOrder,
) *CheckoutCart {
	return &CheckoutCart{
		store:         store,
		createOrderUC: createOrderUC,
		db:            db,
	}
}

// cartClearer is an optional extension satisfied by PostgresStore in production.
// It allows cart clearing to participate in the caller's DB transaction without
// referencing any concrete infrastructure type in this package.
// MemoryStore (dev/test) does not implement it; the fallback path is used.
type cartClearer interface {
	ClearWithTx(ctx context.Context, tx *gorm.DB, key string, barbershopID uint) error
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

	orderInput := ucOrder.CreateOrderInput{
		BarbershopID: input.BarbershopID,
		ClientID:     nil,
		Items:        items,
	}

	// Transactional path: CreateOrder and cart Clear run in the same DB
	// transaction, eliminating the window where the order exists but the cart
	// has not been cleared yet. PostgresStore satisfies cartClearer; MemoryStore
	// does not and falls through to the legacy path below.
	if cc, ok := uc.store.(cartClearer); ok {
		var createdOrder *orderDomain.Order

		err := uc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			order, err := uc.createOrderUC.WithTx(tx).Execute(ctx, orderInput)
			if err != nil {
				return err
			}
			createdOrder = order
			return cc.ClearWithTx(ctx, tx, cartKey, input.BarbershopID)
		})
		if err != nil {
			return nil, err
		}

		return createdOrder, nil
	}

	// Fallback for non-transactional stores (MemoryStore in dev/test).
	order, err := uc.createOrderUC.Execute(ctx, orderInput)
	if err != nil {
		return nil, err
	}
	if err := uc.store.Clear(ctx, cartKey, input.BarbershopID); err != nil {
		return nil, err
	}
	return order, nil
}
