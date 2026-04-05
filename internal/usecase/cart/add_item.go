package cart

import (
	"context"
	"errors"
	"strings"

	domainCart "github.com/BruksfildServices01/barber-scheduler/internal/domain/cart"
	domainProduct "github.com/BruksfildServices01/barber-scheduler/internal/domain/product"
	"github.com/BruksfildServices01/barber-scheduler/internal/dto"
)

var (
	ErrInvalidCartKey     = errors.New("invalid_cart_key")
	ErrInvalidBarbershop  = errors.New("invalid_barbershop_id")
	ErrInvalidProductID   = errors.New("invalid_product_id")
	ErrInvalidQuantity    = errors.New("invalid_quantity")
	ErrProductNotFound    = errors.New("product_not_found")
	ErrProductUnavailable = errors.New("product_unavailable")
)

type CartStore interface {
	Get(ctx context.Context, key string, barbershopID uint) (*domainCart.Cart, error)
	Save(ctx context.Context, cart *domainCart.Cart) error
	RemoveItem(ctx context.Context, key string, barbershopID uint, productID uint) (*domainCart.Cart, error)
	Clear(ctx context.Context, key string, barbershopID uint) error
}

type AddItemInput struct {
	CartKey      string
	BarbershopID uint
	ProductID    uint
	Quantity     int
}

type AddItem struct {
	store       CartStore
	productRepo domainProduct.Repository
}

func NewAddItem(
	store CartStore,
	productRepo domainProduct.Repository,
) *AddItem {
	return &AddItem{
		store:       store,
		productRepo: productRepo,
	}
}

func (uc *AddItem) Execute(
	ctx context.Context,
	input AddItemInput,
) (*dto.PublicCartDTO, error) {
	cartKey := strings.TrimSpace(input.CartKey)
	if cartKey == "" {
		return nil, ErrInvalidCartKey
	}
	if input.BarbershopID == 0 {
		return nil, ErrInvalidBarbershop
	}
	if input.ProductID == 0 {
		return nil, ErrInvalidProductID
	}
	if input.Quantity < 1 {
		return nil, ErrInvalidQuantity
	}

	product, err := uc.productRepo.GetByID(ctx, input.BarbershopID, input.ProductID)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, ErrProductNotFound
	}
	if !product.Active || !product.OnlineVisible || product.Stock <= 0 {
		return nil, ErrProductUnavailable
	}

	cart, err := uc.store.Get(ctx, cartKey, input.BarbershopID)
	if err != nil {
		return nil, err
	}
	if cart == nil {
		cart = domainCart.New(cartKey, input.BarbershopID)
	}

	found := false
	for i := range cart.Items {
		if cart.Items[i].ProductID == input.ProductID {
			newQty := cart.Items[i].Quantity + input.Quantity
			cart.Items[i] = domainCart.NewItem(
				product.ID,
				product.Name,
				newQty,
				product.Price,
			)
			found = true
			break
		}
	}

	if !found {
		cart.Items = append(cart.Items, domainCart.NewItem(
			product.ID,
			product.Name,
			input.Quantity,
			product.Price,
		))
	}

	cart.RecalculateTotals()

	if err := uc.store.Save(ctx, cart); err != nil {
		return nil, err
	}

	return toPublicCartDTO(cart), nil
}

func toPublicCartDTO(cart *domainCart.Cart) *dto.PublicCartDTO {
	items := make([]dto.PublicCartItemDTO, 0, len(cart.Items))
	for _, item := range cart.Items {
		items = append(items, dto.PublicCartItemDTO{
			ProductID:      item.ProductID,
			ProductName:    item.ProductName,
			Quantity:       item.Quantity,
			UnitPriceCents: item.UnitPriceCents,
			LineTotalCents: item.LineTotalCents,
		})
	}

	return &dto.PublicCartDTO{
		Key:           cart.Key,
		Items:         items,
		SubtotalCents: cart.SubtotalCents,
		TotalCents:    cart.TotalCents,
	}
}
