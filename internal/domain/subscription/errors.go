package subscription

import "errors"

// Erros sentinela retornados pelo repository de assinatura.
// Definidos no domínio para que usecases possam usar errors.Is sem importar infra.
var (
	ErrActiveSubscriptionNotFound = errors.New("active_subscription_not_found")
	ErrCutsLimitExceeded          = errors.New("cuts_limit_exceeded")
)
