package metrics

import (
	"context"

	domainMetrics "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
	domainSubscription "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

type ClientWithCategory struct {
	ClientID uint
	Category domainMetrics.ClientCategory
}

type GetClientsWithCategory struct {
	repo             domainMetrics.ClientMetricsRepository
	subscriptionRepo domainSubscription.Repository
}

func NewGetClientsWithCategory(
	repo domainMetrics.ClientMetricsRepository,
	subscriptionRepo domainSubscription.Repository,
) *GetClientsWithCategory {
	return &GetClientsWithCategory{
		repo:             repo,
		subscriptionRepo: subscriptionRepo,
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
		} else {
			// 2) premium por assinatura ativa
			sub, err := uc.subscriptionRepo.GetActiveSubscription(ctx, barbershopID, m.ClientID)
			if err != nil {
				return nil, err
			}
			if sub != nil && sub.Status == domainSubscription.StatusActive {
				category = domainMetrics.CategoryPremium
			}
		}

		out = append(out, ClientWithCategory{
			ClientID: m.ClientID,
			Category: category,
		})
	}

	return out, nil
}
