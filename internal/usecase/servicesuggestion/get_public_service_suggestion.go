package servicesuggestion

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/servicesuggestion"
)

type GetPublicServiceSuggestion struct {
	repo domain.Repository
}

func NewGetPublicServiceSuggestion(
	repo domain.Repository,
) *GetPublicServiceSuggestion {
	return &GetPublicServiceSuggestion{
		repo: repo,
	}
}

type GetPublicServiceSuggestionInput struct {
	BarbershopID uint
	ServiceID    uint
}

func (uc *GetPublicServiceSuggestion) Execute(
	ctx context.Context,
	input GetPublicServiceSuggestionInput,
) (*domain.ServiceSuggestion, error) {
	if input.BarbershopID == 0 || input.ServiceID == 0 {
		return nil, domain.ErrInvalidContext
	}

	return uc.repo.GetPublicSuggestionByServiceID(
		ctx,
		input.BarbershopID,
		input.ServiceID,
	)
}
