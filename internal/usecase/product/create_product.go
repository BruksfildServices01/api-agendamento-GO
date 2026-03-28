package product

import (
	"context"
	"errors"
	"strings"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/product"
)

type CreateProduct struct {
	repo domain.Repository
}

func NewCreateProduct(repo domain.Repository) *CreateProduct {
	return &CreateProduct{repo: repo}
}

type CreateProductInput struct {
	BarbershopID  uint
	Name          string
	Description   string
	Category      string
	Price         int64
	Stock         int
	Active        bool
	OnlineVisible bool
}

func (uc *CreateProduct) Execute(
	ctx context.Context,
	input CreateProductInput,
) (*domain.Product, error) {
	if input.BarbershopID == 0 {
		return nil, errors.New("invalid_barbershop_id")
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("invalid_name")
	}

	if input.Price < 0 {
		return nil, errors.New("invalid_price")
	}

	if input.Stock < 0 {
		return nil, errors.New("invalid_stock")
	}

	if input.OnlineVisible && input.Stock <= 0 {
		return nil, errors.New("invalid_online_visible_without_stock")
	}

	product := &domain.Product{
		BarbershopID:  input.BarbershopID,
		Name:          name,
		Description:   strings.TrimSpace(input.Description),
		Category:      strings.TrimSpace(input.Category),
		Price:         input.Price,
		Stock:         input.Stock,
		Active:        input.Active,
		OnlineVisible: input.OnlineVisible,
	}

	if err := uc.repo.Create(ctx, product); err != nil {
		return nil, err
	}

	return product, nil
}
