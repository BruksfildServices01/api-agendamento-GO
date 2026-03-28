package subscription

import (
	"context"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

type ActivateSubscription struct {
	repo domain.Repository
}

func NewActivateSubscription(repo domain.Repository) *ActivateSubscription {
	return &ActivateSubscription{repo: repo}
}

type ActivateSubscriptionInput struct {
	BarbershopID uint
	ClientID     uint
	PlanID       uint
}

func (uc *ActivateSubscription) Execute(
	ctx context.Context,
	input ActivateSubscriptionInput,
) error {
	if input.BarbershopID == 0 || input.ClientID == 0 || input.PlanID == 0 {
		return ErrActivateSubscriptionInvalidInput
	}

	activeSub, err := uc.repo.GetActiveSubscription(
		ctx,
		input.BarbershopID,
		input.ClientID,
	)
	if err != nil {
		return err
	}
	if activeSub != nil {
		return ErrActivateSubscriptionClientAlreadyHasActiveSub
	}

	plan, err := uc.repo.GetPlanByID(ctx, input.BarbershopID, input.PlanID)
	if err != nil {
		return err
	}
	if plan == nil {
		return ErrActivateSubscriptionPlanNotFound
	}
	if !plan.Active {
		return ErrActivateSubscriptionPlanInactive
	}
	if plan.DurationDays <= 0 {
		return ErrActivateSubscriptionInvalidPlanDuration
	}

	now := time.Now().UTC()

	sub := &domain.Subscription{
		BarbershopID:       input.BarbershopID,
		ClientID:           input.ClientID,
		PlanID:             input.PlanID,
		Status:             domain.StatusActive,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 0, plan.DurationDays),
		CutsUsedInPeriod:   0,
		Plan:               plan,
	}

	if err := uc.repo.ActivateSubscription(ctx, sub); err != nil {
		return err
	}

	return nil
}
