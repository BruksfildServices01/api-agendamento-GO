package cart

import (
	"context"
	"errors"
	"strings"

	domainCart "github.com/BruksfildServices01/barber-scheduler/internal/domain/cart"
	"github.com/BruksfildServices01/barber-scheduler/internal/dto"
)

var ErrGetCartInvalidKey = errors.New("invalid_cart_key")
var ErrGetCartInvalidBarbershop = errors.New("invalid_barbershop_id")

type GetCartInput struct {
	CartKey      string
	BarbershopID uint
}

type GetCart struct {
	store CartStore
}

func NewGetCart(store CartStore) *GetCart {
	return &GetCart{store: store}
}

func (uc *GetCart) Execute(
	ctx context.Context,
	input GetCartInput,
) (*dto.PublicCartDTO, error) {
	cartKey := strings.TrimSpace(input.CartKey)
	if cartKey == "" {
		return nil, ErrGetCartInvalidKey
	}
	if input.BarbershopID == 0 {
		return nil, ErrGetCartInvalidBarbershop
	}

	cart, err := uc.store.Get(ctx, cartKey, input.BarbershopID)
	if err != nil {
		return nil, err
	}

	if cart == nil {
		cart = domainCart.New(cartKey, input.BarbershopID)
	}

	return toPublicCartDTO(cart), nil
}
