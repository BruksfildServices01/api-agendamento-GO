package subscription

import (
	"errors"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

var (
	ErrInvalidInput                       = errors.New("invalid_input")
	ErrPlanHasActiveSubscriptions         = errors.New("plan_has_active_subscriptions")
	ErrPlanNotFound                       = errors.New("plan_not_found")
	ErrPlanInactive                       = errors.New("plan_inactive")
	ErrInvalidPlanDuration                = errors.New("invalid_plan_duration")
	ErrClientAlreadyHasActiveSubscription = errors.New("client_already_has_active_subscription")
	// Aponta para o domínio — errors.Is funciona com domain.ErrActiveSubscriptionNotFound
	ErrActiveSubscriptionNotFound = domain.ErrActiveSubscriptionNotFound
	ErrCutsLimitExceeded          = domain.ErrCutsLimitExceeded
	ErrInvalidBarbershop          = errors.New("invalid_barbershop")
	ErrInvalidName                        = errors.New("invalid_name")
	ErrInvalidPrice                       = errors.New("invalid_price")
	ErrInvalidDurationDays                = errors.New("invalid_duration_days")
	ErrInvalidCutsIncluded                = errors.New("invalid_cuts_included")
	ErrInvalidDiscount                    = errors.New("invalid_discount")
	ErrServiceIDsRequired                 = errors.New("service_ids_required")
	ErrInvalidServiceID                   = errors.New("invalid_service_id")
	ErrInvalidServiceIDs                  = errors.New("invalid_service_ids")

	ErrActivateSubscriptionInvalidInput              = ErrInvalidInput
	ErrActivateSubscriptionPlanNotFound              = ErrPlanNotFound
	ErrActivateSubscriptionPlanInactive              = ErrPlanInactive
	ErrActivateSubscriptionInvalidPlanDuration       = ErrInvalidPlanDuration
	ErrActivateSubscriptionClientAlreadyHasActiveSub = ErrClientAlreadyHasActiveSubscription
)
