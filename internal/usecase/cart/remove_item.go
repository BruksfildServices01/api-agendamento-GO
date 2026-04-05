package cart

import (
	"context"
	"errors"
	"strings"

	domainCart "github.com/BruksfildServices01/barber-scheduler/internal/domain/cart"
	"github.com/BruksfildServices01/barber-scheduler/internal/dto"
)

var ErrRemoveItemInvalidKey = errors.New("invalid_cart_key")
var ErrRemoveItemInvalidBarbershop = errors.New("invalid_barbershop_id")
var ErrRemoveItemInvalidProduct = errors.New("invalid_product_id")

type RemoveItemInput struct {
	CartKey      string
	BarbershopID uint
	ProductID    uint
}

type RemoveItem struct {
	store CartStore
}

func NewRemoveItem(store CartStore) *RemoveItem {
	return &RemoveItem{store: store}
}

func (uc *RemoveItem) Execute(
	ctx context.Context,
	input RemoveItemInput,
) (*dto.PublicCartDTO, error) {
	cartKey := strings.TrimSpace(input.CartKey)
	if cartKey == "" {
		return nil, ErrRemoveItemInvalidKey
	}
	if input.BarbershopID == 0 {
		return nil, ErrRemoveItemInvalidBarbershop
	}
	if input.ProductID == 0 {
		return nil, ErrRemoveItemInvalidProduct
	}

	cart, err := uc.store.RemoveItem(
		ctx,
		cartKey,
		input.BarbershopID,
		input.ProductID,
	)
	if err != nil {
		return nil, err
	}

	if cart == nil {
		cart = domainCart.New(cartKey, input.BarbershopID)
	}

	return toPublicCartDTO(cart), nil
}
