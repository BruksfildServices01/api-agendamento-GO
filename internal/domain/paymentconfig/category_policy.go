package paymentconfig

import (
	"errors"

	domainMetrics "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
)

// ======================================================
// ENTITY
// ======================================================

type CategoryPaymentPolicy struct {
	BarbershopID uint
	Category     domainMetrics.ClientCategory
	Requirement  PaymentRequirement
}

type CategoryPaymentPolicies []CategoryPaymentPolicy

// ======================================================
// DOMAIN LOGIC
// ======================================================

// Resolve qual regra aplicar para uma categoria.
// Se não existir regra específica, cai no default.
func (policies CategoryPaymentPolicies) RequirementFor(
	category domainMetrics.ClientCategory,
	defaultRequirement PaymentRequirement,
) PaymentRequirement {

	for _, p := range policies {
		if p.Category == category {
			return p.Requirement
		}
	}

	return defaultRequirement
}

// ======================================================
// VALIDATION
// ======================================================

var (
	ErrInvalidClientCategory     = errors.New("invalid_client_category")
	ErrInvalidPaymentRequirement = errors.New("invalid_payment_requirement")
)

// Validação da entidade (invariante de domínio)
func (p CategoryPaymentPolicy) Validate() error {
	if !isValidClientCategory(p.Category) {
		return ErrInvalidClientCategory
	}

	if !IsValidRequirement(p.Requirement) {
		return ErrInvalidPaymentRequirement
	}

	return nil
}

// ======================================================
// INTERNAL HELPERS (domínio puro)
// ======================================================

func isValidClientCategory(c domainMetrics.ClientCategory) bool {
	switch c {
	case
		domainMetrics.CategoryNew,
		domainMetrics.CategoryRegular,
		domainMetrics.CategoryAtRisk,
		domainMetrics.CategoryTrusted:
		return true
	default:
		return false
	}
}
