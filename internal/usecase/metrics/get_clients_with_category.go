package metrics

import (
	"context"

	domainMetrics "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
)

type ClientWithCategory struct {
	ClientID uint
	Category domainMetrics.ClientCategory
}

type GetClientsWithCategory struct {
	repo domainMetrics.ClientMetricsRepository
}

func NewGetClientsWithCategory(
	repo domainMetrics.ClientMetricsRepository,
) *GetClientsWithCategory {
	return &GetClientsWithCategory{
		repo: repo,
	}
}

func (uc *GetClientsWithCategory) Execute(
	ctx context.Context,
	barbershopID uint,
) ([]ClientWithCategory, error) {

	metricsList, err := uc.repo.FindByBarbershop(ctx, barbershopID)
	if err != nil {
		return nil, err
	}

	out := make([]ClientWithCategory, 0, len(metricsList))

	for _, m := range metricsList {
		category := domainMetrics.Classify(m)

		// 1) override manual
		if m.CategorySource == domainMetrics.CategorySourceManual && m.Category != "" {
			category = m.Category
		}

		out = append(out, ClientWithCategory{
			ClientID: m.ClientID,
			Category: category,
		})
	}

	return out, nil
}
