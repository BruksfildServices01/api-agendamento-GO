package appointment

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
)

type GetOperationalSummary struct {
	repo domain.Repository
}

func NewGetOperationalSummary(
	repo domain.Repository,
) *GetOperationalSummary {
	return &GetOperationalSummary{repo: repo}
}

func (uc *GetOperationalSummary) Execute(
	ctx context.Context,
	barbershopID uint,
) (*domain.OperationalSummary, error) {

	return uc.repo.GetOperationalSummary(ctx, barbershopID)
}
