package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	domainMetrics "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
	"github.com/BruksfildServices01/barber-scheduler/internal/domain/paymentconfig"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type BarbershopPaymentConfigGormRepository struct {
	db *gorm.DB
}

func NewBarbershopPaymentConfigGormRepository(
	db *gorm.DB,
) *BarbershopPaymentConfigGormRepository {
	return &BarbershopPaymentConfigGormRepository{db: db}
}

//
// ======================================================
// CONFIG (DEFAULT)
// ======================================================
//

func (r *BarbershopPaymentConfigGormRepository) GetByBarbershopID(
	ctx context.Context,
	barbershopID uint,
) (*paymentconfig.Config, error) {

	var m models.BarbershopPaymentConfig

	err := r.db.
		WithContext(ctx).
		Where("barbershop_id = ?", barbershopID).
		First(&m).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// fallback seguro para contas antigas
			return paymentconfig.Default(barbershopID), nil
		}
		return nil, err
	}

	return &paymentconfig.Config{
		BarbershopID:         m.BarbershopID,
		DefaultRequirement:   paymentconfig.PaymentRequirement(m.DefaultRequirement),
		PixExpirationMinutes: m.PixExpirationMinutes,
		MPAccessToken:        m.MPAccessToken,
		MPPublicKey:          m.MPPublicKey,
	}, nil
}

func (r *BarbershopPaymentConfigGormRepository) UpsertConfig(
	ctx context.Context,
	cfg *paymentconfig.Config,
) error {

	var m models.BarbershopPaymentConfig

	err := r.db.
		WithContext(ctx).
		Where("barbershop_id = ?", cfg.BarbershopID).
		First(&m).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// CREATE
			return r.db.WithContext(ctx).Create(&models.BarbershopPaymentConfig{
				BarbershopID:         cfg.BarbershopID,
				DefaultRequirement:   models.PaymentRequirement(cfg.DefaultRequirement),
				PixExpirationMinutes: cfg.PixExpirationMinutes,
				MPAccessToken:        cfg.MPAccessToken,
				MPPublicKey:          cfg.MPPublicKey,
			}).Error
		}
		return err
	}

	// UPDATE
	m.DefaultRequirement = models.PaymentRequirement(cfg.DefaultRequirement)
	m.PixExpirationMinutes = cfg.PixExpirationMinutes
	m.MPAccessToken = cfg.MPAccessToken
	m.MPPublicKey = cfg.MPPublicKey

	return r.db.WithContext(ctx).Save(&m).Error
}

//
// ======================================================
// CATEGORY PAYMENT POLICIES
// ======================================================
//

func (r *BarbershopPaymentConfigGormRepository) ListCategoryPolicies(
	ctx context.Context,
	barbershopID uint,
) ([]paymentconfig.CategoryPaymentPolicy, error) {

	var rows []models.CategoryPaymentPolicy

	if err := r.db.
		WithContext(ctx).
		Where("barbershop_id = ?", barbershopID).
		Find(&rows).
		Error; err != nil {
		return nil, err
	}

	policies := make([]paymentconfig.CategoryPaymentPolicy, 0, len(rows))

	for _, row := range rows {
		policies = append(policies, paymentconfig.CategoryPaymentPolicy{
			BarbershopID: row.BarbershopID,
			Category:     domainMetrics.ClientCategory(row.Category),
			Requirement:  paymentconfig.PaymentRequirement(row.Requirement),
		})
	}

	return policies, nil
}

func (r *BarbershopPaymentConfigGormRepository) UpsertCategoryPolicy(
	ctx context.Context,
	barbershopID uint,
	policy paymentconfig.CategoryPaymentPolicy,
) error {

	row := models.CategoryPaymentPolicy{
		BarbershopID: barbershopID,
		Category:     models.ClientCategory(policy.Category),
		Requirement:  models.PaymentRequirement(policy.Requirement),
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "barbershop_id"},
				{Name: "category"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"requirement",
				"updated_at",
			}),
		}).
		Create(&row).
		Error
}

// ✅ Opção B (PUT como fonte da verdade)
func (r *BarbershopPaymentConfigGormRepository) DeleteCategoryPolicies(
	ctx context.Context,
	barbershopID uint,
) error {

	return r.db.WithContext(ctx).
		Where("barbershop_id = ?", barbershopID).
		Delete(&models.CategoryPaymentPolicy{}).
		Error
}

var _ paymentconfig.Repository = (*BarbershopPaymentConfigGormRepository)(nil)
