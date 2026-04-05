package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/servicesuggestion"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type ServiceSuggestionGormRepository struct {
	db *gorm.DB
}

func NewServiceSuggestionGormRepository(db *gorm.DB) *ServiceSuggestionGormRepository {
	return &ServiceSuggestionGormRepository{db: db}
}

func (r *ServiceSuggestionGormRepository) SetSuggestion(
	ctx context.Context,
	barbershopID uint,
	serviceID uint,
	productID uint,
) error {
	if barbershopID == 0 || serviceID == 0 || productID == 0 {
		return domain.ErrInvalidContext
	}

	// 1) valida serviço no tenant
	var service models.BarbershopService
	if err := r.db.WithContext(ctx).
		Where("id = ? AND barbershop_id = ?", serviceID, barbershopID).
		First(&service).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrServiceNotFound
		}
		return err
	}

	// 2) valida produto no tenant
	var product models.Product
	if err := r.db.WithContext(ctx).
		Where("id = ? AND barbershop_id = ?", productID, barbershopID).
		First(&product).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrProductNotFound
		}
		return err
	}

	// 3) produto inválido para sugestão
	if !product.Active {
		return domain.ErrInvalidSuggestedProduct
	}
	if !product.OnlineVisible {
		return domain.ErrInvalidSuggestedProduct
	}
	if product.Stock <= 0 {
		return domain.ErrInvalidSuggestedProduct
	}

	// 4) regra da sprint: 1 sugestão por serviço
	// se já existir, substitui
	var current models.ServiceSuggestedProduct
	err := r.db.WithContext(ctx).
		Where("barbershop_id = ? AND service_id = ?", barbershopID, serviceID).
		First(&current).Error

	if err == nil {
		current.ProductID = productID
		current.Active = true
		return r.db.WithContext(ctx).Save(&current).Error
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// 5) cria nova associação
	row := models.ServiceSuggestedProduct{
		BarbershopID: barbershopID,
		ServiceID:    serviceID,
		ProductID:    productID,
		Active:       true,
	}

	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *ServiceSuggestionGormRepository) GetSuggestionByServiceID(
	ctx context.Context,
	barbershopID uint,
	serviceID uint,
) (*domain.ServiceSuggestion, error) {
	if barbershopID == 0 || serviceID == 0 {
		return nil, domain.ErrInvalidContext
	}

	var row models.ServiceSuggestedProduct
	err := r.db.WithContext(ctx).
		Preload("Product").
		Where("barbershop_id = ? AND service_id = ? AND active = ?", barbershopID, serviceID, true).
		First(&row).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return mapServiceSuggestionToDomain(&row), nil
}

func (r *ServiceSuggestionGormRepository) RemoveSuggestion(
	ctx context.Context,
	barbershopID uint,
	serviceID uint,
) error {
	if barbershopID == 0 || serviceID == 0 {
		return domain.ErrInvalidContext
	}

	res := r.db.WithContext(ctx).
		Model(&models.ServiceSuggestedProduct{}).
		Where("barbershop_id = ? AND service_id = ?", barbershopID, serviceID).
		Update("active", false)

	if res.Error != nil {
		return res.Error
	}

	return nil
}

func (r *ServiceSuggestionGormRepository) GetPublicSuggestionByServiceID(
	ctx context.Context,
	barbershopID uint,
	serviceID uint,
) (*domain.ServiceSuggestion, error) {
	if barbershopID == 0 || serviceID == 0 {
		return nil, domain.ErrInvalidContext
	}

	var row models.ServiceSuggestedProduct
	err := r.db.WithContext(ctx).
		Preload("Product").
		Joins("JOIN products ON products.id = service_suggested_products.product_id").
		Where("service_suggested_products.barbershop_id = ?", barbershopID).
		Where("service_suggested_products.service_id = ?", serviceID).
		Where("service_suggested_products.active = ?", true).
		Where("products.barbershop_id = ?", barbershopID).
		Where("products.active = ?", true).
		Where("products.online_visible = ?", true).
		Where("products.stock > 0").
		First(&row).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return mapServiceSuggestionToDomain(&row), nil
}

func mapServiceSuggestionToDomain(
	row *models.ServiceSuggestedProduct,
) *domain.ServiceSuggestion {
	out := &domain.ServiceSuggestion{
		ID:           row.ID,
		BarbershopID: row.BarbershopID,
		ServiceID:    row.ServiceID,
		ProductID:    row.ProductID,
		Active:       row.Active,
	}

	if row.Product != nil {
		out.Product = &domain.SuggestedProduct{
			ID:            row.Product.ID,
			BarbershopID:  row.Product.BarbershopID,
			Name:          row.Product.Name,
			Description:   row.Product.Description,
			Category:      row.Product.Category,
			Price:         row.Product.Price,
			Stock:         row.Product.Stock,
			Active:        row.Product.Active,
			OnlineVisible: row.Product.OnlineVisible,
		}
	}

	return out
}

var _ domain.Repository = (*ServiceSuggestionGormRepository)(nil)
