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
	// Only show paid orders (includes closure-linked orders from legacy pending data).
	baseWhere := `
		orders.barbershop_id = ?
		AND (
		    orders.status = 'paid'
		    OR EXISTS (
		        SELECT 1 FROM appointment_closures ac
		        WHERE ac.additional_order_id = orders.id
		    )
		)
	`

	var total int64
	if err := r.db.WithContext(ctx).
		Model(&models.Order{}).
		Where(baseWhere, barbershopID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderBy := buildAdminOrderBy(params.SortBy, params.SortOrder)
	offset := (params.Page - 1) * params.Limit

	type orderListRow struct {
		ID          uint      `gorm:"column:id"`
		Status      string    `gorm:"column:status"`
		OrderSource string    `gorm:"column:order_source"`
		ItemsCount  int       `gorm:"column:items_count"`
		TotalCents  int64     `gorm:"column:total_cents"`
		ClientName  string    `gorm:"column:client_name"`
		ServiceName string    `gorm:"column:service_name"`
		CreatedAt   time.Time `gorm:"column:created_at"`
	}

	var rows []orderListRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			o.id,
			o.status,
			CASE WHEN ac.id IS NOT NULL THEN 'suggestion' ELSE 'standalone' END AS order_source,
			COUNT(DISTINCT oi.id) AS items_count,
			o.total_amount AS total_cents,
			COALESCE(c.name, '') AS client_name,
			COALESCE(
			    NULLIF(ac.actual_service_name, ''),
			    NULLIF(ac.service_name, ''),
			    ''
			) AS service_name,
			o.created_at
		FROM orders o
		LEFT JOIN order_items oi ON oi.order_id = o.id
		LEFT JOIN clients c ON c.id = o.client_id
		LEFT JOIN appointment_closures ac ON ac.additional_order_id = o.id
		WHERE o.barbershop_id = ?
		  AND (
		      o.status = 'paid'
		      OR ac.id IS NOT NULL
		  )
		GROUP BY o.id, ac.id, c.name
		ORDER BY `+orderBy+`
		LIMIT ? OFFSET ?
	`, barbershopID, params.Limit, offset).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	out := make([]dto.OrderListItemDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, dto.OrderListItemDTO{
			ID:          row.ID,
			Status:      row.Status,
			OrderSource: row.OrderSource,
			ItemsCount:  row.ItemsCount,
			TotalCents:  row.TotalCents,
			ClientName:  row.ClientName,
			ServiceName: row.ServiceName,
			CreatedAt:   row.CreatedAt,
		})
	}

	return out, int(total), nil
}

// RichOrderDetail contains full order info including client and linked closure.
type RichOrderDetail struct {
	ID          uint
	Status      string
	OrderSource string // "suggestion" | "standalone"

	ClientID    *uint
	ClientName  string
	ClientPhone string
	ClientEmail string

	ServiceName   string // set when order_source = "suggestion"
	PaymentMethod string // from closure
	AppointmentID *uint

	SubtotalAmount int64
	DiscountAmount int64
	TotalAmount    int64
	CreatedAt      time.Time

	Items []domain.OrderItem
}

func (r *OrderGormRepository) GetRichByID(
	ctx context.Context,
	barbershopID uint,
	orderID uint,
) (*RichOrderDetail, error) {
	type headerRow struct {
		ID            uint      `gorm:"column:id"`
		Status        string    `gorm:"column:status"`
		OrderSource   string    `gorm:"column:order_source"`
		ClientID      *uint     `gorm:"column:client_id"`
		ClientName    string    `gorm:"column:client_name"`
		ClientPhone   string    `gorm:"column:client_phone"`
		ClientEmail   string    `gorm:"column:client_email"`
		ServiceName   string    `gorm:"column:service_name"`
		PaymentMethod string    `gorm:"column:payment_method"`
		AppointmentID *uint     `gorm:"column:appointment_id"`
		SubtotalAmount int64    `gorm:"column:subtotal_amount"`
		DiscountAmount int64    `gorm:"column:discount_amount"`
		TotalAmount    int64    `gorm:"column:total_amount"`
		CreatedAt      time.Time `gorm:"column:created_at"`
	}

	var h headerRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			o.id,
			o.status,
			CASE WHEN ac.id IS NOT NULL THEN 'suggestion' ELSE 'standalone' END AS order_source,
			o.client_id,
			COALESCE(c.name, '')  AS client_name,
			COALESCE(c.phone, '') AS client_phone,
			COALESCE(c.email, '') AS client_email,
			COALESCE(NULLIF(ac.actual_service_name, ''), NULLIF(ac.service_name, ''), '') AS service_name,
			COALESCE(ac.payment_method, '') AS payment_method,
			ac.appointment_id,
			o.subtotal_amount,
			o.discount_amount,
			o.total_amount,
			o.created_at
		FROM orders o
		LEFT JOIN clients c ON c.id = o.client_id
		LEFT JOIN appointment_closures ac ON ac.additional_order_id = o.id
		WHERE o.id = ? AND o.barbershop_id = ?
		LIMIT 1
	`, orderID, barbershopID).Scan(&h).Error
	if err != nil {
		return nil, err
	}
	if h.ID == 0 {
		return nil, nil
	}

	// Load items
	var itemModels []models.OrderItem
	if err := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Find(&itemModels).Error; err != nil {
		return nil, err
	}

	items := make([]domain.OrderItem, 0, len(itemModels))
	for _, it := range itemModels {
		items = append(items, domain.OrderItem{
			ID:                  it.ID,
			OrderID:             it.OrderID,
			ProductID:           it.ProductID,
			ProductNameSnapshot: it.ProductNameSnapshot,
			Quantity:            it.Quantity,
			UnitPrice:           it.UnitPrice,
			LineTotal:           it.LineTotal,
		})
	}

	return &RichOrderDetail{
		ID:             h.ID,
		Status:         h.Status,
		OrderSource:    h.OrderSource,
		ClientID:       h.ClientID,
		ClientName:     h.ClientName,
		ClientPhone:    h.ClientPhone,
		ClientEmail:    h.ClientEmail,
		ServiceName:    h.ServiceName,
		PaymentMethod:  h.PaymentMethod,
		AppointmentID:  h.AppointmentID,
		SubtotalAmount: h.SubtotalAmount,
		DiscountAmount: h.DiscountAmount,
		TotalAmount:    h.TotalAmount,
		CreatedAt:      h.CreatedAt,
		Items:          items,
	}, nil
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
		return "o.status " + sortOrder + ", o.id DESC"
	case "total_amount":
		return "o.total_amount " + sortOrder + ", o.id DESC"
	case "created_at":
		fallthrough
	default:
		return "o.created_at " + sortOrder + ", o.id DESC"
	}
}
