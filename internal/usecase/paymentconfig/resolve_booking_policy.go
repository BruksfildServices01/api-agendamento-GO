package paymentconfig

import (
	"context"

	domainMetrics "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/paymentconfig"
)

// ======================================================
// POLICY RESULT (usada pelo agendamento)
// ======================================================

type BookingPaymentPolicy struct {
	PixExpirationMinutes int

	// Regra global default (vem do DB)
	DefaultRequirement domain.PaymentRequirement

	// Regras específicas por categoria comportamental (vem do DB)
	CategoryPolicies domain.CategoryPaymentPolicies

	// Regra comercial para cliente com assinatura ativa.
	// MVP:
	// - reutiliza a policy da categoria "premium" como fonte de verdade
	// - isso preserva compatibilidade sem manter "premium" como categoria comportamental real
	SubscriptionActiveRequirement domain.PaymentRequirement
}

// ======================================================
// RESOLVER (carrega config da barbearia)
// ======================================================

type ResolveBookingPaymentPolicy struct {
	repo domain.Repository
}

func NewResolveBookingPaymentPolicy(
	repo domain.Repository,
) *ResolveBookingPaymentPolicy {
	return &ResolveBookingPaymentPolicy{repo: repo}
}

func (uc *ResolveBookingPaymentPolicy) Execute(
	ctx context.Context,
	barbershopID uint,
) (*BookingPaymentPolicy, error) {

	// 1) Config da barbearia (DB é fonte da verdade)
	cfg, err := uc.repo.GetByBarbershopID(ctx, barbershopID)
	if err != nil {
		return nil, err
	}

	// 2) Policies por categoria (DB é fonte da verdade)
	categories, err := uc.repo.ListCategoryPolicies(ctx, barbershopID)
	if err != nil {
		return nil, err
	}

	categoryPolicies := domain.CategoryPaymentPolicies(categories)

	// MVP:
	// assinatura ativa reutiliza a policy de "premium", se existir.
	subscriptionRequirement := categoryPolicies.RequirementFor(
		domainMetrics.CategoryPremium,
		cfg.DefaultRequirement,
	)

	// 3) Monta resultado sem hardcode espalhado
	return &BookingPaymentPolicy{
		PixExpirationMinutes:          cfg.PixExpirationMinutes,
		DefaultRequirement:            cfg.DefaultRequirement,
		CategoryPolicies:              categoryPolicies,
		SubscriptionActiveRequirement: subscriptionRequirement,
	}, nil
}
