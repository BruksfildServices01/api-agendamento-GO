package repository

import (
	"context"
	"errors"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/bookingpolicy"
	"gorm.io/gorm"
)

type BookingPolicyGormRepository struct {
	db *gorm.DB
}

func NewBookingPolicyGormRepository(db *gorm.DB) *BookingPolicyGormRepository {
	return &BookingPolicyGormRepository{db: db}
}

func (r *BookingPolicyGormRepository) GetByCategory(
	ctx context.Context,
	barbershopID uint,
	category domain.Category,
) (*domain.BookingPolicy, error) {

	var p domain.BookingPolicy

	err := r.db.WithContext(ctx).
		Where("barbershop_id = ? AND category = ?", barbershopID, category).
		First(&p).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &p, err
}

func (r *BookingPolicyGormRepository) Upsert(
	ctx context.Context,
	p *domain.BookingPolicy,
) error {

	return r.db.WithContext(ctx).
		Save(p).Error
}
