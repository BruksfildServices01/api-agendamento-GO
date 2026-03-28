package service

import (
	"context"
	"strings"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/service"
)

type CreateService struct {
	repo domain.Repository
}

func NewCreateService(repo domain.Repository) *CreateService {
	return &CreateService{repo: repo}
}

type CreateServiceInput struct {
	BarbershopID uint
	Name         string
	Description  string
	DurationMin  int
	Price        int64
	Active       bool
	Category     string
}

func (uc *CreateService) Execute(
	ctx context.Context,
	input CreateServiceInput,
) (*domain.Service, error) {
	if input.BarbershopID == 0 {
		return nil, domain.ErrInvalidContext
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, domain.ErrInvalidName
	}

	if input.DurationMin <= 0 {
		return nil, domain.ErrInvalidDuration
	}

	if input.Price < 0 {
		return nil, domain.ErrInvalidPrice
	}

	svc := &domain.Service{
		BarbershopID: input.BarbershopID,
		Name:         name,
		Description:  strings.TrimSpace(input.Description),
		DurationMin:  input.DurationMin,
		Price:        input.Price,
		Active:       input.Active,
		Category:     strings.TrimSpace(input.Category),
	}

	if err := uc.repo.Create(ctx, svc); err != nil {
		return nil, err
	}

	return svc, nil
}
