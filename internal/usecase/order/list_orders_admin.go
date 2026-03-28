package order

import (
	"context"
	"errors"
	"strings"

	"github.com/BruksfildServices01/barber-scheduler/internal/dto"
	infraRepo "github.com/BruksfildServices01/barber-scheduler/internal/infra/repository"
)

var (
	ErrInvalidBarbershopID = errors.New("invalid_barbershop_id")
	ErrInvalidPage         = errors.New("invalid_page")
	ErrInvalidLimit        = errors.New("invalid_limit")
	ErrInvalidSortField    = errors.New("invalid_sort_field")
	ErrInvalidSortOrder    = errors.New("invalid_sort_order")
)

type ListOrdersAdminInput struct {
	BarbershopID uint
	Status       *string
	Page         int
	Limit        int
	SortBy       string
	SortOrder    string
}

type ListOrdersAdminOutput = dto.PaginatedResponse[dto.OrderListItemDTO]

type ListOrdersAdmin struct {
	orderRepository *infraRepo.OrderGormRepository
}

func NewListOrdersAdmin(
	orderRepo *infraRepo.OrderGormRepository,
) *ListOrdersAdmin {
	return &ListOrdersAdmin{
		orderRepository: orderRepo,
	}
}

func (uc *ListOrdersAdmin) Execute(
	ctx context.Context,
	input ListOrdersAdminInput,
) (*ListOrdersAdminOutput, error) {
	if input.BarbershopID == 0 {
		return nil, ErrInvalidBarbershopID
	}

	page := input.Page
	if page == 0 {
		page = 1
	}
	if page < 1 {
		return nil, ErrInvalidPage
	}

	limit := input.Limit
	if limit == 0 {
		limit = 10
	}
	if limit < 1 {
		return nil, ErrInvalidLimit
	}
	if limit > 100 {
		limit = 100
	}

	sortBy := normalizeSortBy(input.SortBy)
	if sortBy == "" {
		return nil, ErrInvalidSortField
	}

	sortOrder := normalizeSortOrder(input.SortOrder)
	if sortOrder == "" {
		return nil, ErrInvalidSortOrder
	}

	items, total, err := uc.orderRepository.ListAdminByBarbershop(
		ctx,
		input.BarbershopID,
		infraRepo.ListOrdersAdminParams{
			Status:    input.Status,
			Page:      page,
			Limit:     limit,
			SortBy:    sortBy,
			SortOrder: sortOrder,
		},
	)
	if err != nil {
		return nil, err
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}

	return &ListOrdersAdminOutput{
		Data:       items,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

func normalizeSortBy(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "created_at":
		return "created_at"
	case "total_amount":
		return "total_amount"
	case "status":
		return "status"
	default:
		return ""
	}
}

func normalizeSortOrder(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "desc":
		return "desc"
	case "asc":
		return "asc"
	default:
		return ""
	}
}
