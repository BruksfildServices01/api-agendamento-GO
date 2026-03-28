package service

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/service"
)

type ListServices struct {
	repo domain.Repository
}

func NewListServices(repo domain.Repository) *ListServices {
	return &ListServices{repo: repo}
}

func (uc *ListServices) Execute(
	ctx context.Context,
	barbershopID uint,
) ([]*domain.Service, error) {
	if barbershopID == 0 {
		return nil, domain.ErrInvalidContext
	}

	return uc.repo.ListByBarbershop(ctx, barbershopID)
}
