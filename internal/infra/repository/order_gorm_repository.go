package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/order"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type OrderGormRepository struct {
	db *gorm.DB
}

func NewOrderGormRepository(db *gorm.DB) *OrderGormRepository {
	return &OrderGormRepository{db: db}
}

// WithTx retorna um repo “amarrado” à transação atual.
// O usecase deve chamar repo.WithTx(tx) para garantir atomicidade.
func (r *OrderGormRepository) WithTx(tx *gorm.DB) *OrderGormRepository {
	return &OrderGormRepository{db: tx}
}

// ======================================================
// CREATE (TX-SAFE)
// ======================================================
//
// IMPORTANT:
// - NÃO abre Transaction aqui.
// - Assume que o caller (usecase) já abriu a TX quando precisar atomicidade.
func (r *OrderGormRepository) Create(
	ctx context.Context,
	o *domain.Order,
) error {

	db := r.db.WithContext(ctx)

	// 1) Cria Order (sem auto-association)
	orderModel := &models.Order{
		ID:           o.ID,
		BarbershopID: o.BarbershopID,
		Type:         models.OrderType(o.Type),
		Status:       models.OrderStatus(o.Status),
		TotalAmount:  o.TotalAmount,
	}

	if err := db.Create(orderModel).Error; err != nil {
		return err
	}

	// Atualiza ID gerado
	o.ID = orderModel.ID

	// 2) Cria itens (em lote)
	if len(o.Items) > 0 {
		items := make([]models.OrderItem, 0, len(o.Items))
		for _, it := range o.Items {
			items = append(items, models.OrderItem{
				OrderID:   orderModel.ID,
				ItemID:    it.ItemID,
				ItemName:  it.ItemName,
				Quantity:  it.Quantity,
				UnitPrice: it.UnitPrice,
				Total:     it.Total,
			})
		}

		if err := db.Create(&items).Error; err != nil {
			return err
		}
	}

	return nil
}

// ======================================================
// GET BY ID
// ======================================================
func (r *OrderGormRepository) GetByID(
	ctx context.Context,
	barbershopID uint,
	id uint,
) (*domain.Order, error) {

	var m models.Order

	err := r.db.WithContext(ctx).
		Preload("Items").
		Where("id = ? AND barbershop_id = ?", id, barbershopID).
		First(&m).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return mapOrderToDomain(&m), nil
}

func mapOrderToDomain(m *models.Order) *domain.Order {
	items := make([]domain.OrderItem, 0, len(m.Items))

	for _, i := range m.Items {
		items = append(items, domain.OrderItem{
			ItemID:    i.ItemID,
			ItemName:  i.ItemName,
			Quantity:  i.Quantity,
			UnitPrice: i.UnitPrice,
			Total:     i.Total,
		})
	}

	return &domain.Order{
		ID:           m.ID,
		BarbershopID: m.BarbershopID,
		Type:         domain.OrderType(m.Type),
		Status:       domain.OrderStatus(m.Status),
		TotalAmount:  m.TotalAmount,
		Items:        items,
	}
}
