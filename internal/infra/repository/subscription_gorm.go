package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type SubscriptionGormRepository struct {
	db *gorm.DB
}

func NewSubscriptionGormRepository(db *gorm.DB) *SubscriptionGormRepository {
	return &SubscriptionGormRepository{db: db}
}

func (r *SubscriptionGormRepository) CreatePlan(ctx context.Context, plan *domain.Plan) error {
	model := models.Plan{
		BarbershopID:      plan.BarbershopID,
		Name:              plan.Name,
		MonthlyPriceCents: plan.MonthlyPriceCents,
		CutsIncluded:      plan.CutsIncluded,
		DiscountPercent:   plan.DiscountPercent,
		Active:            true,
	}
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *SubscriptionGormRepository) ListPlans(ctx context.Context, barbershopID uint) ([]domain.Plan, error) {
	var modelsPlans []models.Plan

	err := r.db.WithContext(ctx).
		Where("barbershop_id = ?", barbershopID).
		Find(&modelsPlans).Error
	if err != nil {
		return nil, err
	}

	plans := make([]domain.Plan, 0, len(modelsPlans))
	for _, p := range modelsPlans {
		plans = append(plans, domain.Plan{
			ID:                p.ID,
			BarbershopID:      p.BarbershopID,
			Name:              p.Name,
			MonthlyPriceCents: p.MonthlyPriceCents,
			CutsIncluded:      p.CutsIncluded,
			DiscountPercent:   p.DiscountPercent,
			Active:            p.Active,
		})
	}
	return plans, nil
}

func (r *SubscriptionGormRepository) ActivateSubscription(
	ctx context.Context,
	sub *domain.Subscription,
) error {

	model := models.Subscription{
		BarbershopID:       sub.BarbershopID,
		ClientID:           sub.ClientID,
		PlanID:             sub.PlanID,
		Status:             string(sub.Status),
		CurrentPeriodStart: sub.CurrentPeriodStart,
		CurrentPeriodEnd:   sub.CurrentPeriodEnd,
		CutsUsedInPeriod:   sub.CutsUsedInPeriod,
	}

	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *SubscriptionGormRepository) CancelSubscription(
	ctx context.Context,
	barbershopID, clientID uint,
) error {

	return r.db.WithContext(ctx).
		Model(&models.Subscription{}).
		Where(
			"barbershop_id = ? AND client_id = ? AND status = ?",
			barbershopID,
			clientID,
			"active",
		).
		Update("status", string(domain.StatusCancelled)).
		Error
}

func (r *SubscriptionGormRepository) GetActiveSubscription(
	ctx context.Context,
	barbershopID, clientID uint,
) (*domain.Subscription, error) {

	var model models.Subscription

	err := r.db.WithContext(ctx).
		Where(
			"barbershop_id = ? AND client_id = ? AND status = ?",
			barbershopID,
			clientID,
			"active",
		).
		First(&model).Error

	// ✅ FIX: "não encontrado" não é erro — retorna nil, nil
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &domain.Subscription{
		ID:                 model.ID,
		BarbershopID:       model.BarbershopID,
		ClientID:           model.ClientID,
		PlanID:             model.PlanID,
		Status:             domain.Status(model.Status),
		CurrentPeriodStart: model.CurrentPeriodStart,
		CurrentPeriodEnd:   model.CurrentPeriodEnd,
		CutsUsedInPeriod:   model.CutsUsedInPeriod,
	}, nil
}

func (r *SubscriptionGormRepository) IncrementCutsUsed(
	ctx context.Context,
	barbershopID uint,
	clientID uint,
) error {

	return r.db.WithContext(ctx).
		Model(&models.Subscription{}).
		Where(
			"barbershop_id = ? AND client_id = ? AND status = ?",
			barbershopID,
			clientID,
			"active",
		).
		UpdateColumn(
			"cuts_used_in_period",
			gorm.Expr("cuts_used_in_period + 1"),
		).Error
}

func (r *SubscriptionGormRepository) AddServiceToPlan(
	ctx context.Context,
	planID,
	serviceID uint,
) error {

	return r.db.WithContext(ctx).
		Exec(
			`INSERT INTO plan_services (plan_id, service_id)
			 VALUES (?, ?)
			 ON CONFLICT DO NOTHING`,
			planID,
			serviceID,
		).Error
}

func (r *SubscriptionGormRepository) ListAllowedServiceIDs(
	ctx context.Context,
	planID uint,
) ([]uint, error) {

	var ids []uint

	err := r.db.WithContext(ctx).
		Raw(
			`SELECT service_id
			 FROM plan_services
			 WHERE plan_id = ?`,
			planID,
		).
		Scan(&ids).Error

	return ids, err
}

func (r *SubscriptionGormRepository) UpdateCutsUsed(
	ctx context.Context,
	subscriptionID uint,
	newValue int,
) error {

	return r.db.WithContext(ctx).
		Model(&models.Subscription{}).
		Where("id = ?", subscriptionID).
		Update("cuts_used_in_period", newValue).
		Error
}
