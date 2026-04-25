package repository

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"golang.org/x/sync/singleflight"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
)

var errClientAlreadyHasActiveSubscription = errors.New("client_already_has_active_subscription")

// planCacheTTL define por quanto tempo um plano é mantido em memória.
// Planos mudam raramente; 60s elimina o N+1 no hot path de agendamento
// sem risco de dados stale que causem problemas reais.
const planCacheTTL = 60 * time.Second

type planCacheEntry struct {
	plan      *domain.Plan
	expiresAt time.Time
}

var (
	planCacheMu sync.RWMutex
	planCache   = make(map[uint]*planCacheEntry)
	planSFGroup singleflight.Group
)

type SubscriptionGormRepository struct {
	db *gorm.DB
}

func NewSubscriptionGormRepository(db *gorm.DB) *SubscriptionGormRepository {
	return &SubscriptionGormRepository{db: db}
}

// WithTx retorna um repo vinculado à transação tx.
// Permite que operações de assinatura participem de transações externas.
func (r *SubscriptionGormRepository) WithTx(tx *gorm.DB) *SubscriptionGormRepository {
	return &SubscriptionGormRepository{db: tx}
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

	if len(modelsPlans) == 0 {
		return []domain.Plan{}, nil
	}

	// Collect plan IDs for batch loading — avoids N+1 queries.
	planIDs := make([]uint, len(modelsPlans))
	for i, p := range modelsPlans {
		planIDs[i] = p.ID
	}

	// Batch load: plan_services → resolve to barbershop_service IDs.
	type planServiceRow struct {
		PlanID    uint
		ServiceID uint
	}
	var planServiceRows []planServiceRow
	err = r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT ps.plan_id, bs.id AS service_id
		FROM plan_services ps
		JOIN barbershop_services bs ON bs.id = ps.service_id
		WHERE ps.plan_id IN ?
		UNION
		SELECT DISTINCT pc.plan_id, bs.id AS service_id
		FROM plan_categories pc
		JOIN barbershop_services bs ON bs.category_id = pc.category_id
		WHERE pc.plan_id IN ?
	`, planIDs, planIDs).Scan(&planServiceRows).Error
	if err != nil {
		return nil, err
	}

	// Batch load: plan_categories.
	type planCategoryRow struct {
		PlanID     uint
		CategoryID uint
	}
	var planCategoryRows []planCategoryRow
	err = r.db.WithContext(ctx).Raw(
		`SELECT plan_id, category_id FROM plan_categories WHERE plan_id IN ?`,
		planIDs,
	).Scan(&planCategoryRows).Error
	if err != nil {
		return nil, err
	}

	// Index results by plan ID.
	servicesByPlan := make(map[uint][]uint, len(planIDs))
	for _, row := range planServiceRows {
		servicesByPlan[row.PlanID] = append(servicesByPlan[row.PlanID], row.ServiceID)
	}
	categoriesByPlan := make(map[uint][]uint, len(planIDs))
	for _, row := range planCategoryRows {
		categoriesByPlan[row.PlanID] = append(categoriesByPlan[row.PlanID], row.CategoryID)
	}

	plans := make([]domain.Plan, 0, len(modelsPlans))
	for _, p := range modelsPlans {
		plans = append(plans, domain.Plan{
			ID:                p.ID,
			BarbershopID:      p.BarbershopID,
			Name:              p.Name,
			MonthlyPriceCents: p.MonthlyPriceCents,
			DurationDays:      p.DurationDays,
			CutsIncluded:      p.CutsIncluded,
			DiscountPercent:   p.DiscountPercent,
			Active:            p.Active,
			ServiceIDs:        servicesByPlan[p.ID],
			CategoryIDs:       categoriesByPlan[p.ID],
		})
	}

	return plans, nil
}

func (r *SubscriptionGormRepository) GetPlanByID(
	ctx context.Context,
	barbershopID uint,
	planID uint,
) (*domain.Plan, error) {
	// Cache hit: evita as 3 queries adicionais (plan + services + categories)
	// no hot path de agendamento. TTL curto garante que mudanças de plano
	// se propagam em até planCacheTTL segundos.
	planCacheMu.RLock()
	entry, ok := planCache[planID]
	validHit := ok && time.Now().Before(entry.expiresAt)
	planCacheMu.RUnlock()

	if validHit {
		// Valida barbershop_id mesmo no cache hit para multi-tenant safety.
		if entry.plan != nil && entry.plan.BarbershopID != barbershopID {
			return nil, nil
		}
		return entry.plan, nil
	}

	// singleflight: múltiplas goroutines aguardando pelo mesmo planID
	// disparam apenas uma query ao banco.
	sfKey := "plan:" + string(rune(planID))
	v, err, _ := planSFGroup.Do(sfKey, func() (any, error) {
		var model models.Plan
		if err := r.db.WithContext(ctx).
			Where("id = ? AND barbershop_id = ?", planID, barbershopID).
			First(&model).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return (*domain.Plan)(nil), nil
			}
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

		plan := &domain.Plan{
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
		}

		planCacheMu.Lock()
		planCache[planID] = &planCacheEntry{plan: plan, expiresAt: time.Now().Add(planCacheTTL)}
		planCacheMu.Unlock()

		return plan, nil
	})

	if err != nil {
		return nil, err
	}
	return v.(*domain.Plan), nil
}

// evictPlanCache remove a entrada do plano do cache. Deve ser chamado
// após mutações de plano (UpdatePlan, SetPlanActive, DeletePlan).
func evictPlanCache(planID uint) {
	planCacheMu.Lock()
	delete(planCache, planID)
	planCacheMu.Unlock()
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
		return domain.ErrActiveSubscriptionNotFound
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
		ID:                   model.ID,
		BarbershopID:         model.BarbershopID,
		ClientID:             model.ClientID,
		PlanID:               model.PlanID,
		Status:               domain.Status(model.Status),
		CurrentPeriodStart:   model.CurrentPeriodStart,
		CurrentPeriodEnd:     model.CurrentPeriodEnd,
		CutsUsedInPeriod:     model.CutsUsedInPeriod,
		CutsReservedInPeriod: model.CutsReservedInPeriod,
		Plan:                 planPtr,
	}, nil
}

func (r *SubscriptionGormRepository) IncrementCutsUsed(
	ctx context.Context,
	barbershopID uint,
	clientID uint,
) error {
	now := time.Now().UTC()

	// Incremento atômico: só executa se o total (usados + reservados + 1) couber no
	// plano. A subquery evita a race condition de stale-read no nível da aplicação.
	res := r.db.WithContext(ctx).Exec(`
		UPDATE subscriptions
		SET cuts_used_in_period = cuts_used_in_period + 1
		WHERE barbershop_id = ?
		  AND client_id = ?
		  AND status = 'active'
		  AND current_period_start <= ?
		  AND current_period_end > ?
		  AND cuts_used_in_period + cuts_reserved_in_period + 1
		      <= (SELECT cuts_included FROM plans WHERE id = plan_id)
	`, barbershopID, clientID, now, now)

	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		return r.classifyZeroRows(ctx, barbershopID, clientID, false)
	}

	return nil
}

func (r *SubscriptionGormRepository) ReserveSubscriptionCut(
	ctx context.Context,
	barbershopID uint,
	clientID uint,
) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).
		Model(&models.Subscription{}).
		Where(
			`barbershop_id = ? AND client_id = ? AND status = ?
			 AND current_period_start <= ? AND current_period_end > ?`,
			barbershopID, clientID, "active", now, now,
		).
		UpdateColumn("cuts_reserved_in_period", gorm.Expr("cuts_reserved_in_period + 1"))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrActiveSubscriptionNotFound
	}
	return nil
}

func (r *SubscriptionGormRepository) ReleaseSubscriptionCut(
	ctx context.Context,
	barbershopID uint,
	clientID uint,
) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).
		Model(&models.Subscription{}).
		Where(
			`barbershop_id = ? AND client_id = ? AND status = ?
			 AND current_period_start <= ? AND current_period_end > ?
			 AND cuts_reserved_in_period > 0`,
			barbershopID, clientID, "active", now, now,
		).
		UpdateColumn("cuts_reserved_in_period", gorm.Expr("cuts_reserved_in_period - 1"))
	if res.Error != nil {
		return res.Error
	}
	// RowsAffected=0 é OK: período pode ter expirado entre booking e cancelamento.
	return nil
}

func (r *SubscriptionGormRepository) ConsumeReservedCut(
	ctx context.Context,
	barbershopID uint,
	clientID uint,
) error {
	now := time.Now().UTC()

	// Consumo atômico: converte reserva em uso apenas se o total couber no plano.
	// A subquery elimina a race condition de stale-read.
	res := r.db.WithContext(ctx).Exec(`
		UPDATE subscriptions
		SET cuts_used_in_period     = cuts_used_in_period + 1,
		    cuts_reserved_in_period = cuts_reserved_in_period - 1
		WHERE barbershop_id = ?
		  AND client_id = ?
		  AND status = 'active'
		  AND current_period_start <= ?
		  AND current_period_end > ?
		  AND cuts_reserved_in_period > 0
		  AND cuts_used_in_period + 1
		      <= (SELECT cuts_included FROM plans WHERE id = plan_id)
	`, barbershopID, clientID, now, now)

	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		reason := r.classifyZeroRows(ctx, barbershopID, clientID, true)
		if reason == domain.ErrCutsLimitExceeded {
			return domain.ErrCutsLimitExceeded
		}
		// Período expirou entre booking e conclusão — tenta incremento direto.
		return r.IncrementCutsUsed(ctx, barbershopID, clientID)
	}

	return nil
}

// classifyZeroRows determina por que um UPDATE de crédito afetou 0 linhas.
// hasReservation=true indica que o UPDATE exigia cuts_reserved_in_period > 0.
func (r *SubscriptionGormRepository) classifyZeroRows(
	ctx context.Context,
	barbershopID uint,
	clientID uint,
	hasReservation bool,
) error {
	now := time.Now().UTC()

	// Verifica se existe assinatura ativa no período (sem checar o cap).
	q := r.db.WithContext(ctx).
		Model(&models.Subscription{}).
		Where(
			`barbershop_id = ? AND client_id = ? AND status = 'active'
			 AND current_period_start <= ? AND current_period_end > ?`,
			barbershopID, clientID, now, now,
		)

	if hasReservation {
		q = q.Where("cuts_reserved_in_period > 0")
	}

	var count int64
	if err := q.Count(&count).Error; err != nil || count == 0 {
		return domain.ErrActiveSubscriptionNotFound
	}

	// Assinatura existe e período é válido — o UPDATE falhou por cap atingido.
	return domain.ErrCutsLimitExceeded
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

// ──────────────────────────────────────────────────────────────────
// Purchase flow
// ──────────────────────────────────────────────────────────────────

func (r *SubscriptionGormRepository) CreatePendingSubscription(
	ctx context.Context,
	sub *domain.Subscription,
) error {
	model := models.Subscription{
		BarbershopID: sub.BarbershopID,
		ClientID:     sub.ClientID,
		PlanID:       sub.PlanID,
		Status:       string(domain.StatusPendingPayment),
		// period dates are zero — will be set on activation
	}

	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isPendingSubscriptionUniqueViolation(err) {
			return errClientAlreadyHasActiveSubscription
		}
		return err
	}

	sub.ID = model.ID
	return nil
}

func (r *SubscriptionGormRepository) GetSubscriptionByID(
	ctx context.Context,
	id uint,
) (*domain.Subscription, error) {
	var model models.Subscription

	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	plan, err := r.GetPlanByID(ctx, model.BarbershopID, model.PlanID)
	if err != nil {
		return nil, err
	}

	return &domain.Subscription{
		ID:                   model.ID,
		BarbershopID:         model.BarbershopID,
		ClientID:             model.ClientID,
		PlanID:               model.PlanID,
		Status:               domain.Status(model.Status),
		CurrentPeriodStart:   model.CurrentPeriodStart,
		CurrentPeriodEnd:     model.CurrentPeriodEnd,
		CutsUsedInPeriod:     model.CutsUsedInPeriod,
		CutsReservedInPeriod: model.CutsReservedInPeriod,
		Plan:                 plan,
	}, nil
}

func (r *SubscriptionGormRepository) ActivateSubscriptionByID(
	ctx context.Context,
	id uint,
	periodStart, periodEnd time.Time,
) error {
	res := r.db.WithContext(ctx).
		Model(&models.Subscription{}).
		Where("id = ? AND status = ?", id, string(domain.StatusPendingPayment)).
		Updates(map[string]any{
			"status":               string(domain.StatusActive),
			"current_period_start": periodStart,
			"current_period_end":   periodEnd,
			"cuts_used_in_period":  0,
			"cuts_reserved_in_period": 0,
		})

	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrActiveSubscriptionNotFound
	}
	return nil
}

func isPendingSubscriptionUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" && pgErr.ConstraintName == "uq_subscriptions_one_pending_per_client_shop" {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "uq_subscriptions_one_pending_per_client_shop")
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

func (r *SubscriptionGormRepository) UpdatePlan(
	ctx context.Context,
	barbershopID uint,
	planID uint,
	plan *domain.Plan,
	serviceIDs []uint,
	categoryIDs []uint,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.Plan{}).
			Where("id = ? AND barbershop_id = ?", planID, barbershopID).
			Updates(map[string]any{
				"name":                plan.Name,
				"monthly_price_cents": plan.MonthlyPriceCents,
				"duration_days":       plan.DurationDays,
				"cuts_included":       plan.CutsIncluded,
				"discount_percent":    plan.DiscountPercent,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("plan_not_found")
		}

		if err := tx.Exec(`DELETE FROM plan_services WHERE plan_id = ?`, planID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DELETE FROM plan_categories WHERE plan_id = ?`, planID).Error; err != nil {
			return err
		}

		for _, serviceID := range serviceIDs {
			if err := tx.Exec(
				`INSERT INTO plan_services (plan_id, service_id) VALUES (?, ?)`,
				planID, serviceID,
			).Error; err != nil {
				return err
			}
		}
		for _, categoryID := range categoryIDs {
			if err := tx.Exec(
				`INSERT INTO plan_categories (plan_id, category_id) VALUES (?, ?)`,
				planID, categoryID,
			).Error; err != nil {
				return err
			}
		}

		evictPlanCache(planID)
		return nil
	})
}

func (r *SubscriptionGormRepository) SetPlanActive(
	ctx context.Context,
	barbershopID uint,
	planID uint,
	active bool,
) error {
	result := r.db.WithContext(ctx).Model(&models.Plan{}).
		Where("id = ? AND barbershop_id = ?", planID, barbershopID).
		Update("active", active)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("plan_not_found")
	}
	evictPlanCache(planID)
	return nil
}

func (r *SubscriptionGormRepository) DeletePlan(
	ctx context.Context,
	barbershopID uint,
	planID uint,
) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
	if err == nil {
		evictPlanCache(planID)
	}
	return err
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

func (r *SubscriptionGormRepository) ExpireSubscriptions(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Exec(
		`UPDATE subscriptions
		 SET status = 'expired', cuts_reserved_in_period = 0
		 WHERE status = 'active' AND current_period_end < NOW()`,
	)
	return result.RowsAffected, result.Error
}
