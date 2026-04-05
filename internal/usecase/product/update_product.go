package product

import (
	"context"
	"errors"
	"strings"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/product"
)

type UpdateProduct struct {
	repo domain.Repository
}

func NewUpdateProduct(repo domain.Repository) *UpdateProduct {
	return &UpdateProduct{repo: repo}
}

type UpdateProductInput struct {
	BarbershopID uint
	ProductID    uint

	Name          *string
	Description   *string
	Category      *string
	Price         *int64
	Stock         *int
	Active        *bool
	OnlineVisible *bool
}

func (uc *UpdateProduct) Execute(
	ctx context.Context,
	input UpdateProductInput,
) (*domain.Product, error) {
	if input.BarbershopID == 0 || input.ProductID == 0 {
		return nil, errors.New("invalid_context")
	}

	product, err := uc.repo.GetByID(ctx, input.BarbershopID, input.ProductID)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, domain.ErrProductNotFound
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, errors.New("invalid_name")
		}
		product.Name = name
	}

	if input.Description != nil {
		product.Description = strings.TrimSpace(*input.Description)
	}

	if input.Category != nil {
		product.Category = strings.TrimSpace(*input.Category)
	}

	if input.Price != nil {
		if *input.Price < 0 {
			return nil, errors.New("invalid_price")
		}
		product.Price = *input.Price
	}

	if input.Stock != nil {
		if *input.Stock < 0 {
			return nil, errors.New("invalid_stock")
		}
		product.Stock = *input.Stock
	}

	if input.Active != nil {
		product.Active = *input.Active
	}

	if input.OnlineVisible != nil {
		product.OnlineVisible = *input.OnlineVisible
	}

	// Validação do estado final do agregado
	if product.OnlineVisible && product.Stock <= 0 {
		return nil, errors.New("invalid_online_visible_without_stock")
	}

	if err := uc.repo.Update(ctx, product); err != nil {
		return nil, err
	}

	return product, nil
}
