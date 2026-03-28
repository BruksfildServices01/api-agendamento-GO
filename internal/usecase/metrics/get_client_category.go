package metrics

import (
	"context"

	domainMetrics "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
)

type GetClientCategory struct {
	repo domainMetrics.ClientMetricsRepository
}

func NewGetClientCategory(
	repo domainMetrics.ClientMetricsRepository,
) *GetClientCategory {
	return &GetClientCategory{
		repo: repo,
	}
}

func (uc *GetClientCategory) Execute(
	ctx context.Context,
	barbershopID uint,
	clientID uint,
) (domainMetrics.ClientCategory, error) {

	m, err := uc.repo.GetOrCreate(ctx, barbershopID, clientID)
	if err != nil {
		return "", err
	}

	// 1) override manual continua tendo prioridade
	if m.CategorySource == domainMetrics.CategorySourceManual && m.Category != "" {
		return m.Category, nil
	}

	// 2) fallback para classificação comportamental
	return domainMetrics.Classify(m), nil
}
