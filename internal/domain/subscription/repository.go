package subscription

import "context"

type Repository interface {
	CreatePlan(ctx context.Context, plan *Plan) error
	ListPlans(ctx context.Context, barbershopID uint) ([]Plan, error)

	ActivateSubscription(ctx context.Context, sub *Subscription) error
	CancelSubscription(ctx context.Context, barbershopID, clientID uint) error

	GetActiveSubscription(ctx context.Context, barbershopID, clientID uint) (*Subscription, error)

	IncrementCutsUsed(
		ctx context.Context,
		barbershopID uint,
		clientID uint,
	) error

	AddServiceToPlan(ctx context.Context, planID, serviceID uint) error
	ListAllowedServiceIDs(ctx context.Context, planID uint) ([]uint, error)

	UpdateCutsUsed(ctx context.Context, subscriptionID uint, newValue int) error
}
