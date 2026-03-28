package service

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/service"
)

type ListPublicServices struct {
	repo domain.Repository
}

func NewListPublicServices(repo domain.Repository) *ListPublicServices {
	return &ListPublicServices{
		repo: repo,
	}
}

type ListPublicServicesInput struct {
	BarbershopID uint
	Category     string
	Query        string
}

func (uc *ListPublicServices) Execute(
	ctx context.Context,
	input ListPublicServicesInput,
) ([]*domain.Service, error) {
	if input.BarbershopID == 0 {
		return nil, domain.ErrInvalidContext
	}

	return uc.repo.ListPublicServices(
		ctx,
		input.BarbershopID,
		input.Category,
		input.Query,
	)
}
