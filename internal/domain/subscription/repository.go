package subscription

import (
	"context"
	"time"
)

type Repository interface {
	CreatePlan(ctx context.Context, plan *Plan, serviceIDs []uint, categoryIDs []uint) error
	UpdatePlan(ctx context.Context, barbershopID uint, planID uint, plan *Plan, serviceIDs []uint, categoryIDs []uint) error
	SetPlanActive(ctx context.Context, barbershopID uint, planID uint, active bool) error
	ListPlans(ctx context.Context, barbershopID uint) ([]Plan, error)
	GetPlanByID(ctx context.Context, barbershopID uint, planID uint) (*Plan, error)
	DeletePlan(ctx context.Context, barbershopID uint, planID uint) error
	CountActiveSubscriptionsByPlan(ctx context.Context, planID uint) (int64, error)
	CountActiveSubscribersByPlan(ctx context.Context, planID uint) (int64, error)

	ActivateSubscription(ctx context.Context, sub *Subscription) error
	CancelSubscription(ctx context.Context, barbershopID uint, clientID uint) error
	GetActiveSubscription(ctx context.Context, barbershopID uint, clientID uint) (*Subscription, error)

	// Purchase flow
	CreatePendingSubscription(ctx context.Context, sub *Subscription) error
	GetSubscriptionByID(ctx context.Context, id uint) (*Subscription, error)
	ActivateSubscriptionByID(ctx context.Context, id uint, periodStart, periodEnd time.Time) error

	ExpireSubscriptions(ctx context.Context) (int64, error)
	IncrementCutsUsed(ctx context.Context, barbershopID uint, clientID uint) error
	ReserveSubscriptionCut(ctx context.Context, barbershopID uint, clientID uint) error
	ReleaseSubscriptionCut(ctx context.Context, barbershopID uint, clientID uint) error
	ConsumeReservedCut(ctx context.Context, barbershopID uint, clientID uint) error
	AddServiceToPlan(ctx context.Context, planID uint, serviceID uint) error
	ListAllowedServiceIDs(ctx context.Context, planID uint) ([]uint, error)
	UpdateCutsUsed(ctx context.Context, subscriptionID uint, newValue int) error

	CountServicesByIDs(ctx context.Context, barbershopID uint, serviceIDs []uint) (int64, error)
	CountServicesByBarbershop(ctx context.Context, barbershopID uint, serviceIDs []uint) (int64, error)
	CountCategoriesByIDs(ctx context.Context, barbershopID uint, categoryIDs []uint) (int64, error)
}
