package servicesuggestion

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/servicesuggestion"
)

type GetServiceSuggestion struct {
	repo domain.Repository
}

func NewGetServiceSuggestion(
	repo domain.Repository,
) *GetServiceSuggestion {
	return &GetServiceSuggestion{
		repo: repo,
	}
}

type GetServiceSuggestionInput struct {
	BarbershopID uint
	ServiceID    uint
}

func (uc *GetServiceSuggestion) Execute(
	ctx context.Context,
	input GetServiceSuggestionInput,
) (*domain.ServiceSuggestion, error) {
	if input.BarbershopID == 0 || input.ServiceID == 0 {
		return nil, domain.ErrInvalidContext
	}

	return uc.repo.GetSuggestionByServiceID(
		ctx,
		input.BarbershopID,
		input.ServiceID,
	)
}
