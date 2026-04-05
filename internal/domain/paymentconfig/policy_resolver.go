package paymentconfig

import domainMetrics "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"

// ResolvePaymentRequirement decide a exigência final de pagamento
func ResolvePaymentRequirement(
	globalRequirePix bool,
	category domainMetrics.ClientCategory,
	categoryPolicies CategoryPaymentPolicies,
) PaymentRequirement {

	var defaultRequirement PaymentRequirement

	if globalRequirePix {
		defaultRequirement = PaymentMandatory
	} else {
		defaultRequirement = PaymentOptional
	}

	return categoryPolicies.RequirementFor(
		category,
		defaultRequirement,
	)
}
