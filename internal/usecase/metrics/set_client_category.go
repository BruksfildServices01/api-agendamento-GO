package metrics

import (
	"context"
	"errors"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
)

type SetClientCategory struct {
	repo domain.ClientMetricsRepository
}

func NewSetClientCategory(
	repo domain.ClientMetricsRepository,
) *SetClientCategory {
	return &SetClientCategory{repo: repo}
}

type SetClientCategoryInput struct {
	BarbershopID uint
	ClientID     uint
	Category     domain.ClientCategory
}

func (uc *SetClientCategory) Execute(
	ctx context.Context,
	input SetClientCategoryInput,
) error {

	if input.BarbershopID == 0 || input.ClientID == 0 {
		return errors.New("invalid_context")
	}

	switch input.Category {
	case domain.CategoryNew,
		domain.CategoryRegular,
		domain.CategoryTrusted,
		domain.CategoryAtRisk:
	default:
		return errors.New("invalid_category")
	}
	m, err := uc.repo.GetOrCreate(ctx, input.BarbershopID, input.ClientID)
	if err != nil {
		return err
	}

	m.SetManualCategory(input.Category)

	return uc.repo.Save(ctx, m)
}
