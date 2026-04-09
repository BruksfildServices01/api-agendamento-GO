package product

import (
	"context"
	"errors"
	"strings"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/product"
	"github.com/BruksfildServices01/barber-scheduler/internal/dto"
)

type ListPublicProducts struct {
	repo domain.Repository
}

func NewListPublicProducts(repo domain.Repository) *ListPublicProducts {
	return &ListPublicProducts{repo: repo}
}

type ListPublicProductsInput struct {
	BarbershopID uint
	Category     string
	Query        string
}

func (uc *ListPublicProducts) Execute(
	ctx context.Context,
	input ListPublicProductsInput,
) ([]dto.PublicProductListItemDTO, error) {
	if input.BarbershopID == 0 {
		return nil, errors.New("invalid_barbershop_id")
	}

	products, err := uc.repo.ListPublicProducts(
		ctx,
		input.BarbershopID,
		strings.TrimSpace(strings.ToLower(input.Category)),
		strings.TrimSpace(input.Query),
	)
	if err != nil {
		return nil, err
	}

	out := make([]dto.PublicProductListItemDTO, 0, len(products))
	for _, p := range products {
		out = append(out, dto.PublicProductListItemDTO{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			PriceCents:  p.Price,
			Category:    p.Category,
			ImageURL:    p.ImageURL,
		})
	}

	return out, nil
}
