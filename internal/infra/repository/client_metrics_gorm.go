package repository

import (
	"context"
	"errors"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
	infraModels "github.com/BruksfildServices01/barber-scheduler/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ClientMetricsGormRepository struct {
	db *gorm.DB
}

func NewClientMetricsGormRepository(db *gorm.DB) *ClientMetricsGormRepository {
	return &ClientMetricsGormRepository{db: db}
}

func (r *ClientMetricsGormRepository) GetOrCreate(
	ctx context.Context,
	barbershopID uint,
	clientID uint,
) (*domain.ClientMetrics, error) {

	var m infraModels.ClientMetrics

	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("client_id = ? AND barbershop_id = ?", clientID, barbershopID).
		First(&m).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		m = infraModels.ClientMetrics{
			ClientID:     clientID,
			BarbershopID: barbershopID,
		}

		if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	return mapToDomain(&m), nil
}

func (r *ClientMetricsGormRepository) Save(
	ctx context.Context,
	m *domain.ClientMetrics,
) error {

	model := mapToModel(m)

	return r.db.WithContext(ctx).Save(model).Error
}

func mapToDomain(m *infraModels.ClientMetrics) *domain.ClientMetrics {
	return &domain.ClientMetrics{
		ClientID:     m.ClientID,
		BarbershopID: m.BarbershopID,

		TotalAppointments:     m.TotalAppointments,
		CompletedAppointments: m.CompletedAppointments,
		CancelledAppointments: m.CancelledAppointments,
		NoShowAppointments:    m.NoShowAppointments,

		TotalSpent: m.TotalSpent,

		FirstAppointmentAt: m.FirstAppointmentAt,
		LastAppointmentAt:  m.LastAppointmentAt,
		LastCompletedAt:    m.LastCompletedAt,
		LastCanceledAt:     m.LastCanceledAt,

		Category:       domain.ClientCategory(m.Category),
		CategorySource: domain.CategorySource(m.CategorySource),

		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func mapToModel(m *domain.ClientMetrics) *infraModels.ClientMetrics {
	return &infraModels.ClientMetrics{
		ClientID:     m.ClientID,
		BarbershopID: m.BarbershopID,

		TotalAppointments:     m.TotalAppointments,
		CompletedAppointments: m.CompletedAppointments,
		CancelledAppointments: m.CancelledAppointments,
		NoShowAppointments:    m.NoShowAppointments,

		TotalSpent: m.TotalSpent,

		FirstAppointmentAt: m.FirstAppointmentAt,
		LastAppointmentAt:  m.LastAppointmentAt,
		LastCompletedAt:    m.LastCompletedAt,
		LastCanceledAt:     m.LastCanceledAt,

		Category:       infraModels.ClientCategory(m.Category),           // ✅ conversão correta
		CategorySource: infraModels.CategorySourceType(m.CategorySource), // ✅ conversão correta
	}
}

func (r *ClientMetricsGormRepository) FindByBarbershop(
	ctx context.Context,
	barbershopID uint,
) ([]*domain.ClientMetrics, error) {

	var models []infraModels.ClientMetrics

	if err := r.db.WithContext(ctx).
		Where("barbershop_id = ?", barbershopID).
		Find(&models).Error; err != nil {
		return nil, err
	}

	result := make([]*domain.ClientMetrics, 0, len(models))
	for _, m := range models {
		result = append(result, mapToDomain(&m))
	}

	return result, nil
}
