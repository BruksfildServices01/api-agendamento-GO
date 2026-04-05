package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/order"
	"github.com/BruksfildServices01/barber-scheduler/internal/dto"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type OrderGormRepository struct {
	db *gorm.DB
}

type ListOrdersAdminParams struct {
	Status    *string
	Page      int
	Limit     int
	SortBy    string
	SortOrder string
}

func NewOrderGormRepository(db *gorm.DB) *OrderGormRepository {
	return &OrderGormRepository{db: db}
}

// WithTx retorna um repo amarrado à transação atual.
func (r *OrderGormRepository) WithTx(tx *gorm.DB) *OrderGormRepository {
	return &OrderGormRepository{db: tx}
}

func (r *OrderGormRepository) Create(
	ctx context.Context,
	o *domain.Order,
) error {
	db := r.db.WithContext(ctx)

	orderModel := &models.Order{
		ID:             o.ID,
		BarbershopID:   o.BarbershopID,
		ClientID:       o.ClientID,
		Type:           models.OrderType(o.Type),
		Status:         models.OrderStatus(o.Status),
		SubtotalAmount: o.SubtotalAmount,
		DiscountAmount: o.DiscountAmount,
		TotalAmount:    o.TotalAmount,
	}

	if err := db.Create(orderModel).Error; err != nil {
		return err
	}

	o.ID = orderModel.ID

	if len(o.Items) > 0 {
		items := make([]models.OrderItem, 0, len(o.Items))
		for _, it := range o.Items {
			items = append(items, models.OrderItem{
				OrderID:             orderModel.ID,
				ProductID:           it.ProductID,
				ProductNameSnapshot: it.ProductNameSnapshot,
				Quantity:            it.Quantity,
				UnitPrice:           it.UnitPrice,
				LineTotal:           it.LineTotal,
			})
		}

		if err := db.Create(&items).Error; err != nil {
			return err
		}
	}

	return nil
}

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

func (r *OrderGormRepository) ListByBarbershop(
	ctx context.Context,
	barbershopID uint,
) ([]domain.Order, error) {
	var rows []models.Order

	err := r.db.WithContext(ctx).
		Preload("Items").
		Where("barbershop_id = ?", barbershopID).
		Order("id DESC").
		Find(&rows).
		Error
	if err != nil {
		return nil, err
	}

	out := make([]domain.Order, 0, len(rows))
	for i := range rows {
		mapped := mapOrderToDomain(&rows[i])
		out = append(out, *mapped)
	}

	return out, nil
}

func (r *OrderGormRepository) ListAdminByBarbershop(
	ctx context.Context,
	barbershopID uint,
	params ListOrdersAdminParams,
) ([]dto.OrderListItemDTO, int, error) {
	query := r.db.WithContext(ctx).
		Model(&models.Order{}).
		Where("orders.barbershop_id = ?", barbershopID)

	if params.Status != nil && *params.Status != "" {
		query = query.Where("orders.status = ?", *params.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderBy := buildAdminOrderBy(params.SortBy, params.SortOrder)
	offset := (params.Page - 1) * params.Limit

	type orderListRow struct {
		ID            uint      `gorm:"column:id"`
		Status        string    `gorm:"column:status"`
		ItemsCount    int       `gorm:"column:items_count"`
		TotalCents    int64     `gorm:"column:total_cents"`
		PaymentStatus string    `gorm:"column:payment_status"`
		CreatedAt     time.Time `gorm:"column:created_at"`
	}

	var rows []orderListRow
	err := query.
		Select(`
			orders.id,
			orders.status,
			COUNT(DISTINCT order_items.id) AS items_count,
			orders.total_amount AS total_cents,
			COALESCE(MAX(payments.status::text), '') AS payment_status,
			orders.created_at
		`).
		Joins("LEFT JOIN order_items ON order_items.order_id = orders.id").
		Joins("LEFT JOIN payments ON payments.order_id = orders.id").
		Group("orders.id").
		Order(orderBy).
		Limit(params.Limit).
		Offset(offset).
		Scan(&rows).
		Error
	if err != nil {
		return nil, 0, err
	}

	out := make([]dto.OrderListItemDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, dto.OrderListItemDTO{
			ID:            row.ID,
			Status:        row.Status,
			ItemsCount:    row.ItemsCount,
			TotalCents:    row.TotalCents,
			PaymentStatus: row.PaymentStatus,
			CreatedAt:     row.CreatedAt,
		})
	}

	return out, int(total), nil
}

func mapOrderToDomain(m *models.Order) *domain.Order {
	items := make([]domain.OrderItem, 0, len(m.Items))

	for _, i := range m.Items {
		items = append(items, domain.OrderItem{
			ID:                  i.ID,
			OrderID:             i.OrderID,
			ProductID:           i.ProductID,
			ProductNameSnapshot: i.ProductNameSnapshot,
			Quantity:            i.Quantity,
			UnitPrice:           i.UnitPrice,
			LineTotal:           i.LineTotal,
		})
	}

	return &domain.Order{
		ID:             m.ID,
		BarbershopID:   m.BarbershopID,
		ClientID:       m.ClientID,
		Type:           domain.OrderType(m.Type),
		Status:         domain.OrderStatus(m.Status),
		SubtotalAmount: m.SubtotalAmount,
		DiscountAmount: m.DiscountAmount,
		TotalAmount:    m.TotalAmount,
		Items:          items,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

func buildAdminOrderBy(sortBy, sortOrder string) string {
	switch sortBy {
	case "status":
		return "orders.status " + sortOrder + ", orders.id DESC"
	case "total_amount":
		return "orders.total_amount " + sortOrder + ", orders.id DESC"
	case "created_at":
		fallthrough
	default:
		return "orders.created_at " + sortOrder + ", orders.id DESC"
	}
}
