package service

import (
	"context"
	"strings"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/service"
)

type UpdateService struct {
	repo domain.Repository
}

func NewUpdateService(repo domain.Repository) *UpdateService {
	return &UpdateService{repo: repo}
}

type UpdateServiceInput struct {
	BarbershopID uint
	ServiceID    uint

	Name        *string
	Description *string
	DurationMin *int
	Price       *int64
	Active      *bool
	Category    *string
	CategoryID  *uint
}

func (uc *UpdateService) Execute(
	ctx context.Context,
	input UpdateServiceInput,
) (*domain.Service, error) {
	if input.BarbershopID == 0 || input.ServiceID == 0 {
		return nil, domain.ErrInvalidContext
	}

	svc, err := uc.repo.GetByID(ctx, input.BarbershopID, input.ServiceID)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, domain.ErrServiceNotFound
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, domain.ErrInvalidName
		}
		svc.Name = name
	}

	if input.Description != nil {
		svc.Description = strings.TrimSpace(*input.Description)
	}

	if input.DurationMin != nil {
		if *input.DurationMin <= 0 {
			return nil, domain.ErrInvalidDuration
		}
		svc.DurationMin = *input.DurationMin
	}

	if input.Price != nil {
		if *input.Price < 0 {
			return nil, domain.ErrInvalidPrice
		}
		svc.Price = *input.Price
	}

	if input.Active != nil {
		svc.Active = *input.Active
	}

	if input.Category != nil {
		svc.Category = strings.TrimSpace(*input.Category)
	}

	if input.CategoryID != nil {
		svc.CategoryID = input.CategoryID
	}

	if err := uc.repo.Update(ctx, svc); err != nil {
		return nil, err
	}

	return svc, nil
}
