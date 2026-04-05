package repository

import (
	"context"

	domainMetrics "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/paymentconfig"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ======================================================
// GORM MODEL
// ======================================================

type CategoryPaymentPolicyModel struct {
	BarbershopID uint                             `gorm:"primaryKey"`
	Category     domainMetrics.ClientCategory     `gorm:"primaryKey;size:32"`
	Requirement  domainPayment.PaymentRequirement `gorm:"size:16;not null"`
}

// ======================================================
// REPOSITORY
// ======================================================

type CategoryPaymentPolicyGormRepository struct {
	db *gorm.DB
}

func NewCategoryPaymentPolicyGormRepository(
	db *gorm.DB,
) *CategoryPaymentPolicyGormRepository {
	return &CategoryPaymentPolicyGormRepository{db: db}
}

// ======================================================
// READ
// ======================================================

func (r *CategoryPaymentPolicyGormRepository) ListByBarbershop(
	ctx context.Context,
	barbershopID uint,
) (domainPayment.CategoryPaymentPolicies, error) {

	var rows []CategoryPaymentPolicyModel

	if err := r.db.WithContext(ctx).
		Where("barbershop_id = ?", barbershopID).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	policies := make(domainPayment.CategoryPaymentPolicies, 0, len(rows))
	for _, row := range rows {
		policies = append(policies, domainPayment.CategoryPaymentPolicy{
			Category:    row.Category,
			Requirement: row.Requirement,
		})
	}

	return policies, nil
}

// ======================================================
// UPSERT
// ======================================================

func (r *CategoryPaymentPolicyGormRepository) Upsert(
	ctx context.Context,
	barbershopID uint,
	policy domainPayment.CategoryPaymentPolicy,
) error {

	model := CategoryPaymentPolicyModel{
		BarbershopID: barbershopID,
		Category:     policy.Category,
		Requirement:  policy.Requirement,
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "barbershop_id"},
				{Name: "category"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"requirement",
			}),
		}).
		Create(&model).Error
}
