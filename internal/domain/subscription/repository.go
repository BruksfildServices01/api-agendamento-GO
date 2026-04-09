package subscription

import "context"

type Repository interface {
	CreatePlan(ctx context.Context, plan *Plan, serviceIDs []uint, categoryIDs []uint) error
	ListPlans(ctx context.Context, barbershopID uint) ([]Plan, error)
	GetPlanByID(ctx context.Context, barbershopID uint, planID uint) (*Plan, error)
	DeletePlan(ctx context.Context, barbershopID uint, planID uint) error
	CountActiveSubscriptionsByPlan(ctx context.Context, planID uint) (int64, error)
	CountActiveSubscribersByPlan(ctx context.Context, planID uint) (int64, error)

	ActivateSubscription(ctx context.Context, sub *Subscription) error
	CancelSubscription(ctx context.Context, barbershopID uint, clientID uint) error
	GetActiveSubscription(ctx context.Context, barbershopID uint, clientID uint) (*Subscription, error)

	IncrementCutsUsed(ctx context.Context, barbershopID uint, clientID uint) error
	AddServiceToPlan(ctx context.Context, planID uint, serviceID uint) error
	ListAllowedServiceIDs(ctx context.Context, planID uint) ([]uint, error)
	UpdateCutsUsed(ctx context.Context, subscriptionID uint, newValue int) error

	CountServicesByIDs(ctx context.Context, barbershopID uint, serviceIDs []uint) (int64, error)
	CountServicesByBarbershop(ctx context.Context, barbershopID uint, serviceIDs []uint) (int64, error)
	CountCategoriesByIDs(ctx context.Context, barbershopID uint, categoryIDs []uint) (int64, error)
}
