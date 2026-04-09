package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	errClientAlreadyHasActiveSubscription = errors.New("client_already_has_active_subscription")
	errActiveSubscriptionNotFound         = errors.New("active_subscription_not_found")
)

type SubscriptionGormRepository struct {
	db *gorm.DB
}

func NewSubscriptionGormRepository(db *gorm.DB) *SubscriptionGormRepository {
	return &SubscriptionGormRepository{db: db}
}

func (r *SubscriptionGormRepository) CreatePlan(
	ctx context.Context,
	plan *domain.Plan,
	serviceIDs []uint,
	categoryIDs []uint,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		model := models.Plan{
			BarbershopID:      plan.BarbershopID,
			Name:              plan.Name,
			MonthlyPriceCents: plan.MonthlyPriceCents,
			DurationDays:      plan.DurationDays,
			CutsIncluded:      plan.CutsIncluded,
			DiscountPercent:   plan.DiscountPercent,
			Active:            true,
		}

		if err := tx.Create(&model).Error; err != nil {
			return err
		}

		for _, serviceID := range serviceIDs {
			if err := tx.Exec(
				`INSERT INTO plan_services (plan_id, service_id)
				 VALUES (?, ?)`,
				model.ID,
				serviceID,
			).Error; err != nil {
				return err
			}
		}

		for _, categoryID := range categoryIDs {
			if err := tx.Exec(
				`INSERT INTO plan_categories (plan_id, category_id) VALUES (?, ?)`,
				model.ID, categoryID,
			).Error; err != nil {
				return err
			}
		}

		plan.ID = model.ID
		return nil
	})
}

func (r *SubscriptionGormRepository) ListPlans(
	ctx context.Context,
	barbershopID uint,
) ([]domain.Plan, error) {
	var modelsPlans []models.Plan

	err := r.db.WithContext(ctx).
		Where("barbershop_id = ?", barbershopID).
		Find(&modelsPlans).Error
	if err != nil {
		return nil, err
	}

	plans := make([]domain.Plan, 0, len(modelsPlans))
	for _, p := range modelsPlans {
		serviceIDs, err := r.ListAllowedServiceIDs(ctx, p.ID)
		if err != nil {
			return nil, err
		}

		categoryIDs, err := r.listPlanCategoryIDs(ctx, p.ID)
		if err != nil {
			return nil, err
		}

		plans = append(plans, domain.Plan{
			ID:                p.ID,
			BarbershopID:      p.BarbershopID,
			Name:              p.Name,
			MonthlyPriceCents: p.MonthlyPriceCents,
			DurationDays:      p.DurationDays,
			CutsIncluded:      p.CutsIncluded,
			DiscountPercent:   p.DiscountPercent,
			Active:            p.Active,
			ServiceIDs:        serviceIDs,
			CategoryIDs:       categoryIDs,
		})
	}

	return plans, nil
}

func (r *SubscriptionGormRepository) GetPlanByID(
	ctx context.Context,
	barbershopID uint,
	planID uint,
) (*domain.Plan, error) {
	var model models.Plan

	err := r.db.WithContext(ctx).
		Where("id = ? AND barbershop_id = ?", planID, barbershopID).
		First(&model).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	serviceIDs, err := r.ListAllowedServiceIDs(ctx, model.ID)
	if err != nil {
		return nil, err
	}

	categoryIDs, err := r.listPlanCategoryIDs(ctx, model.ID)
	if err != nil {
		return nil, err
	}

	return &domain.Plan{
		ID:                model.ID,
		BarbershopID:      model.BarbershopID,
		Name:              model.Name,
		MonthlyPriceCents: model.MonthlyPriceCents,
		DurationDays:      model.DurationDays,
		CutsIncluded:      model.CutsIncluded,
		DiscountPercent:   model.DiscountPercent,
		Active:            model.Active,
		ServiceIDs:        serviceIDs,
		CategoryIDs:       categoryIDs,
	}, nil
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

	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isActiveSubscriptionUniqueViolation(err) {
			return errClientAlreadyHasActiveSubscription
		}
		return err
	}

	sub.ID = model.ID
	return nil
}

func (r *SubscriptionGormRepository) CancelSubscription(
	ctx context.Context,
	barbershopID, clientID uint,
) error {
	res := r.db.WithContext(ctx).
		Model(&models.Subscription{}).
		Where(
			"barbershop_id = ? AND client_id = ? AND status = ?",
			barbershopID,
			clientID,
			"active",
		).
		Update("status", string(domain.StatusCancelled))

	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		return errActiveSubscriptionNotFound
	}

	return nil
}

func (r *SubscriptionGormRepository) GetActiveSubscription(
	ctx context.Context,
	barbershopID, clientID uint,
) (*domain.Subscription, error) {
	var model models.Subscription

	now := time.Now().UTC()

	err := r.db.WithContext(ctx).
		Where(
			`barbershop_id = ?
			 AND client_id = ?
			 AND status = ?
			 AND current_period_start <= ?
			 AND current_period_end > ?`,
			barbershopID,
			clientID,
			"active",
			now,
			now,
		).
		Order("current_period_end DESC").
		First(&model).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var planPtr *domain.Plan

	plan, err := r.GetPlanByID(ctx, barbershopID, model.PlanID)
	if err != nil {
		return nil, err
	}
	if plan != nil {
		planPtr = plan
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
		Plan:               planPtr,
	}, nil
}

func (r *SubscriptionGormRepository) IncrementCutsUsed(
	ctx context.Context,
	barbershopID uint,
	clientID uint,
) error {
	now := time.Now().UTC()

	res := r.db.WithContext(ctx).
		Model(&models.Subscription{}).
		Where(
			`barbershop_id = ?
			 AND client_id = ?
			 AND status = ?
			 AND current_period_start <= ?
			 AND current_period_end > ?`,
			barbershopID,
			clientID,
			"active",
			now,
			now,
		).
		UpdateColumn(
			"cuts_used_in_period",
			gorm.Expr("cuts_used_in_period + 1"),
		)

	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		return errActiveSubscriptionNotFound
	}

	return nil
}

func (r *SubscriptionGormRepository) AddServiceToPlan(
	ctx context.Context,
	planID,
	serviceID uint,
) error {
	return r.db.WithContext(ctx).
		Exec(
			`INSERT INTO plan_services (plan_id, service_id)
			 VALUES (?, ?)`,
			planID,
			serviceID,
		).Error
}

func (r *SubscriptionGormRepository) ListAllowedServiceIDs(
	ctx context.Context,
	planID uint,
) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT bs.id
		FROM barbershop_services bs
		WHERE bs.id IN (SELECT service_id FROM plan_services WHERE plan_id = ?)
		   OR (bs.category_id IS NOT NULL AND bs.category_id IN (
		       SELECT category_id FROM plan_categories WHERE plan_id = ?
		   ))
	`, planID, planID).Scan(&ids).Error
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

func isActiveSubscriptionUniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" && pgErr.ConstraintName == "uq_subscriptions_one_active_per_client_shop" {
			return true
		}
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "uq_subscriptions_one_active_per_client_shop")
}

func (r *SubscriptionGormRepository) DeletePlan(
	ctx context.Context,
	barbershopID uint,
	planID uint,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Remove assinaturas não-ativas que referenciam este plano (canceladas/expiradas)
		if err := tx.
			Where("plan_id = ? AND status != ?", planID, "active").
			Delete(&models.Subscription{}).Error; err != nil {
			return err
		}

		if err := tx.Exec(`DELETE FROM plan_categories WHERE plan_id = ?`, planID).Error; err != nil {
			return err
		}

		if err := tx.Exec(
			`DELETE FROM plan_services WHERE plan_id = ?`, planID,
		).Error; err != nil {
			return err
		}

		return tx.
			Where("id = ? AND barbershop_id = ?", planID, barbershopID).
			Delete(&models.Plan{}).Error
	})
}

func (r *SubscriptionGormRepository) CountActiveSubscriptionsByPlan(
	ctx context.Context,
	planID uint,
) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Subscription{}).
		Where("plan_id = ? AND status = ?", planID, "active").
		Count(&count).Error
	return count, err
}

func (r *SubscriptionGormRepository) CountActiveSubscribersByPlan(
	ctx context.Context,
	planID uint,
) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Subscription{}).
		Where("plan_id = ? AND status = ?", planID, "active").
		Count(&count).Error
	return count, err
}

func (r *SubscriptionGormRepository) CountServicesByBarbershop(
	ctx context.Context,
	barbershopID uint,
	serviceIDs []uint,
) (int64, error) {
	return r.CountServicesByIDs(ctx, barbershopID, serviceIDs)
}

func (r *SubscriptionGormRepository) CountServicesByIDs(
	ctx context.Context,
	barbershopID uint,
	serviceIDs []uint,
) (int64, error) {
	if len(serviceIDs) == 0 {
		return 0, nil
	}

	var count int64

	err := r.db.WithContext(ctx).
		Model(&models.BarbershopService{}).
		Where("barbershop_id = ? AND id IN ?", barbershopID, serviceIDs).
		Count(&count).Error
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *SubscriptionGormRepository) listPlanCategoryIDs(ctx context.Context, planID uint) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).Raw(
		`SELECT category_id FROM plan_categories WHERE plan_id = ?`, planID,
	).Scan(&ids).Error
	return ids, err
}

func (r *SubscriptionGormRepository) CountCategoriesByIDs(
	ctx context.Context,
	barbershopID uint,
	categoryIDs []uint,
) (int64, error) {
	if len(categoryIDs) == 0 {
		return 0, nil
	}
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.ServiceCategory{}).
		Where("barbershop_id = ? AND id IN ?", barbershopID, categoryIDs).
		Count(&count).Error
	return count, err
}
