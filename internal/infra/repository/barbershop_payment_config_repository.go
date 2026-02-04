package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

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
			// fallback seguro
			return paymentconfig.Default(barbershopID), nil
		}
		return nil, err
	}

	return &paymentconfig.Config{
		BarbershopID:         m.BarbershopID,
		RequirePixOnBooking:  m.RequirePixOnBooking,
		PixExpirationMinutes: m.PixExpirationMinutes,
	}, nil
}
