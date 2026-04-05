package servicesuggestion

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/servicesuggestion"
)

type RemoveServiceSuggestion struct {
	repo domain.Repository
}

func NewRemoveServiceSuggestion(
	repo domain.Repository,
) *RemoveServiceSuggestion {
	return &RemoveServiceSuggestion{
		repo: repo,
	}
}

type RemoveServiceSuggestionInput struct {
	BarbershopID uint
	ServiceID    uint
}

func (uc *RemoveServiceSuggestion) Execute(
	ctx context.Context,
	input RemoveServiceSuggestionInput,
) error {
	if input.BarbershopID == 0 || input.ServiceID == 0 {
		return domain.ErrInvalidContext
	}

	return uc.repo.RemoveSuggestion(
		ctx,
		input.BarbershopID,
		input.ServiceID,
	)
}
