package metrics

import (
	"context"

	domainMetrics "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
	domainSubscription "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

type GetClientCategory struct {
	repo             domainMetrics.ClientMetricsRepository
	subscriptionRepo domainSubscription.Repository
}

func NewGetClientCategory(
	repo domainMetrics.ClientMetricsRepository,
	subscriptionRepo domainSubscription.Repository,
) *GetClientCategory {
	return &GetClientCategory{
		repo:             repo,
		subscriptionRepo: subscriptionRepo,
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

	// 2) premium agora vem de assinatura ativa
	sub, err := uc.subscriptionRepo.GetActiveSubscription(ctx, barbershopID, clientID)
	if err != nil {
		return "", err
	}
	if sub != nil && sub.Status == domainSubscription.StatusActive {
		return domainMetrics.CategoryPremium, nil
	}

	// 3) fallback para classificação comportamental
	return domainMetrics.Classify(m), nil
}
