package servicesuggestion

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/servicesuggestion"
)

type SetServiceSuggestion struct {
	repo domain.Repository
}

func NewSetServiceSuggestion(
	repo domain.Repository,
) *SetServiceSuggestion {
	return &SetServiceSuggestion{
		repo: repo,
	}
}

type SetServiceSuggestionInput struct {
	BarbershopID uint
	ServiceID    uint
	ProductID    uint
}

func (uc *SetServiceSuggestion) Execute(
	ctx context.Context,
	input SetServiceSuggestionInput,
) error {
	if input.BarbershopID == 0 || input.ServiceID == 0 || input.ProductID == 0 {
		return domain.ErrInvalidContext
	}

	return uc.repo.SetSuggestion(
		ctx,
		input.BarbershopID,
		input.ServiceID,
		input.ProductID,
	)
}
