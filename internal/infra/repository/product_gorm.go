package repository

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/product"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type ProductGormRepository struct {
	db *gorm.DB
}

func NewProductGormRepository(db *gorm.DB) *ProductGormRepository {
	return &ProductGormRepository{db: db}
}

// WithTx retorna um repo “amarrado” à transação atual.
// Assim, o usecase consegue garantir atomicidade (stock + order) numa única TX.
func (r *ProductGormRepository) WithTx(tx *gorm.DB) *ProductGormRepository {
	return &ProductGormRepository{db: tx}
}

func (r *ProductGormRepository) Create(
	ctx context.Context,
	p *domain.Product,
) error {
	model := mapProductToModel(p)

	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}

	p.ID = model.ID
	return nil
}

func (r *ProductGormRepository) Update(
	ctx context.Context,
	p *domain.Product,
) error {
	model := mapProductToModel(p)

	return r.db.WithContext(ctx).
		Model(&models.Product{}).
		Where("id = ? AND barbershop_id = ?", p.ID, p.BarbershopID).
		Updates(map[string]any{
			"name":           model.Name,
			"description":    model.Description,
			"category":       model.Category,
			"price":          model.Price,
			"stock":          model.Stock,
			"active":         model.Active,
			"online_visible": model.OnlineVisible,
		}).
		Error
}

func (r *ProductGormRepository) Delete(
	ctx context.Context,
	barbershopID uint,
	id uint,
) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND barbershop_id = ?", id, barbershopID).
		Delete(&models.Product{}).
		Error
}

func (r *ProductGormRepository) GetByID(
	ctx context.Context,
	barbershopID uint,
	id uint,
) (*domain.Product, error) {
	var m models.Product

	err := r.db.WithContext(ctx).
		Where("id = ? AND barbershop_id = ?", id, barbershopID).
		First(&m).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return mapProductToDomain(&m), nil
}

func (r *ProductGormRepository) ListByBarbershop(
	ctx context.Context,
	barbershopID uint,
) ([]*domain.Product, error) {
	var modelsList []models.Product

	err := r.db.WithContext(ctx).
		Where("barbershop_id = ?", barbershopID).
		Order("created_at DESC, id DESC").
		Find(&modelsList).
		Error
	if err != nil {
		return nil, err
	}

	result := make([]*domain.Product, 0, len(modelsList))
	for i := range modelsList {
		result = append(result, mapProductToDomain(&modelsList[i]))
	}

	return result, nil
}

func (r *ProductGormRepository) ListPublicProducts(
	ctx context.Context,
	barbershopID uint,
	category string,
	query string,
) ([]*domain.Product, error) {
	var modelsList []models.Product

	category = strings.ToLower(strings.TrimSpace(category))
	query = strings.ToLower(strings.TrimSpace(query))

	q := r.db.WithContext(ctx).
		Where(
			"barbershop_id = ? AND active = ? AND online_visible = ? AND stock > 0",
			barbershopID,
			true,
			true,
		)

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
		Order("created_at DESC, id DESC").
		Find(&modelsList).
		Error
	if err != nil {
		return nil, err
	}

	result := make([]*domain.Product, 0, len(modelsList))
	for i := range modelsList {
		result = append(result, mapProductToDomain(&modelsList[i]))
	}

	return result, nil
}

// DecreaseStock decrementa estoque de forma atômica e segura.
// Requer que o repo esteja com a TX correta (use r.WithTx(tx) no usecase).
func (r *ProductGormRepository) DecreaseStock(
	ctx context.Context,
	barbershopID uint,
	productID uint,
	quantity int,
) error {
	if quantity <= 0 {
		return errors.New("invalid_quantity")
	}

	result := r.db.WithContext(ctx).
		Model(&models.Product{}).
		Where(
			"id = ? AND barbershop_id = ? AND stock >= ?",
			productID,
			barbershopID,
			quantity,
		).
		Update("stock", gorm.Expr("stock - ?", quantity))

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return domain.ErrInsufficientStock
	}

	return nil
}

func mapProductToDomain(m *models.Product) *domain.Product {
	return &domain.Product{
		ID:            m.ID,
		BarbershopID:  m.BarbershopID,
		Name:          m.Name,
		Description:   m.Description,
		Category:      m.Category,
		Price:         m.Price,
		Stock:         m.Stock,
		Active:        m.Active,
		OnlineVisible: m.OnlineVisible,
	}
}

func mapProductToModel(p *domain.Product) *models.Product {
	return &models.Product{
		ID:            p.ID,
		BarbershopID:  p.BarbershopID,
		Name:          p.Name,
		Description:   p.Description,
		Category:      p.Category,
		Price:         p.Price,
		Stock:         p.Stock,
		Active:        p.Active,
		OnlineVisible: p.OnlineVisible,
	}
}

var _ domain.Repository = (*ProductGormRepository)(nil)
