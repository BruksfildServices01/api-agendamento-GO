package metrics

import (
	"context"
	"time"

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

	// 1) override manual tem prioridade se ainda não expirou
	if m.CategorySource == domainMetrics.CategorySourceManual && m.Category != "" {
		if m.ManualCategoryExpiresAt == nil || time.Now().UTC().Before(*m.ManualCategoryExpiresAt) {
			return m.Category, nil
		}
	}

	// 2) fallback para classificação comportamental
	return domainMetrics.Classify(m), nil
}
