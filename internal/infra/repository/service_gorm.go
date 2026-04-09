package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/service"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type ServiceGormRepository struct {
	db *gorm.DB
}

func NewServiceGormRepository(db *gorm.DB) *ServiceGormRepository {
	return &ServiceGormRepository{db: db}
}

func (r *ServiceGormRepository) Create(
	ctx context.Context,
	s *domain.Service,
) error {
	model := mapServiceToModel(s)

	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}

	s.ID = model.ID
	return nil
}

func (r *ServiceGormRepository) Update(
	ctx context.Context,
	s *domain.Service,
) error {
	model := mapServiceToModel(s)

	return r.db.WithContext(ctx).
		Model(&models.BarbershopService{}).
		Where("id = ? AND barbershop_id = ?", s.ID, s.BarbershopID).
		Updates(map[string]any{
			"name":         model.Name,
			"description":  model.Description,
			"duration_min": model.DurationMin,
			"price":        model.Price,
			"active":       model.Active,
			"category":     model.Category,
			"category_id":  model.CategoryID,
		}).
		Error
}

func (r *ServiceGormRepository) GetByID(
	ctx context.Context,
	barbershopID uint,
	id uint,
) (*domain.Service, error) {
	var model models.BarbershopService

	err := r.db.WithContext(ctx).
		Where("id = ? AND barbershop_id = ?", id, barbershopID).
		First(&model).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return mapServiceToDomain(&model), nil
}

func (r *ServiceGormRepository) ListByBarbershop(
	ctx context.Context,
	barbershopID uint,
) ([]*domain.Service, error) {
	var modelsList []models.BarbershopService

	err := r.db.WithContext(ctx).
		Where("barbershop_id = ?", barbershopID).
		Order("created_at DESC").
		Find(&modelsList).
		Error
	if err != nil {
		return nil, err
	}

	result := make([]*domain.Service, 0, len(modelsList))
	for i := range modelsList {
		result = append(result, mapServiceToDomain(&modelsList[i]))
	}

	return result, nil
}

func (r *ServiceGormRepository) ListActiveByBarbershop(
	ctx context.Context,
	barbershopID uint,
) ([]*domain.Service, error) {
	var modelsList []models.BarbershopService

	err := r.db.WithContext(ctx).
		Where("barbershop_id = ? AND active = ?", barbershopID, true).
		Order("created_at DESC").
		Find(&modelsList).
		Error
	if err != nil {
		return nil, err
	}

	result := make([]*domain.Service, 0, len(modelsList))
	for i := range modelsList {
		result = append(result, mapServiceToDomain(&modelsList[i]))
	}

	return result, nil
}

func (r *ServiceGormRepository) ListPublicServices(
	ctx context.Context,
	barbershopID uint,
	category string,
	query string,
) ([]*domain.Service, error) {
	var modelsList []models.BarbershopService

	q := r.db.WithContext(ctx).
		Preload("ServiceImages", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Where("barbershop_id = ? AND active = ?", barbershopID, true)

	if category != "" {
		q = q.Where("LOWER(category) = ?", category)
	}

	if query != "" {
		like := "%" + query + "%"
		q = q.Where(
			"(LOWER(name) LIKE ? OR LOWER(description) LIKE ?)",
			like,
			like,
		)
	}

	err := q.
		Order("created_at DESC").
		Find(&modelsList).
		Error
	if err != nil {
		return nil, err
	}

	result := make([]*domain.Service, 0, len(modelsList))
	for i := range modelsList {
		result = append(result, mapServiceToDomain(&modelsList[i]))
	}

	return result, nil
}

func mapServiceToDomain(m *models.BarbershopService) *domain.Service {
	imgs := make([]domain.ServiceImage, 0, len(m.ServiceImages))
	for _, img := range m.ServiceImages {
		imgs = append(imgs, domain.ServiceImage{
			ID:       img.ID,
			URL:      img.URL,
			Position: img.Position,
		})
	}
	return &domain.Service{
		ID:           m.ID,
		BarbershopID: m.BarbershopID,
		Name:         m.Name,
		Description:  m.Description,
		DurationMin:  m.DurationMin,
		Price:        m.Price,
		Active:       m.Active,
		Category:     m.Category,
		CategoryID:   m.CategoryID,
		Images:       imgs,
	}
}

func mapServiceToModel(s *domain.Service) *models.BarbershopService {
	return &models.BarbershopService{
		ID:           s.ID,
		BarbershopID: s.BarbershopID,
		Name:         s.Name,
		Description:  s.Description,
		DurationMin:  s.DurationMin,
		Price:        s.Price,
		Active:       s.Active,
		Category:     s.Category,
		CategoryID:   s.CategoryID,
	}
}

var _ domain.Repository = (*ServiceGormRepository)(nil)
