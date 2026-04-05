package cart

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/cart"
)

// cartRow maps to the carts table.
type cartRow struct {
	Key          string    `gorm:"primaryKey;column:key"`
	BarbershopID uint      `gorm:"primaryKey;column:barbershop_id"`
	Items        string    `gorm:"column:items;type:jsonb"`
	ExpiresAt    time.Time `gorm:"column:expires_at"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (cartRow) TableName() string { return "carts" }

// cartTTL defines how long a cart lives without interaction.
const cartTTL = 24 * time.Hour

// PostgresStore is a persistent cart store backed by PostgreSQL.
// Replaces MemoryStore to support multi-instance deployments.
type PostgresStore struct {
	db *gorm.DB
}

func NewPostgresStore(db *gorm.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) Get(ctx context.Context, key string, barbershopID uint) (*domain.Cart, error) {
	var row cartRow
	err := s.db.WithContext(ctx).
		Where("key = ? AND barbershop_id = ? AND expires_at > ?", key, barbershopID, time.Now().UTC()).
		First(&row).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return rowToCart(row)
}

func (s *PostgresStore) Save(ctx context.Context, cart *domain.Cart) error {
	itemsJSON, err := json.Marshal(cart.Items)
	if err != nil {
		return err
	}

	row := cartRow{
		Key:          cart.Key,
		BarbershopID: cart.BarbershopID,
		Items:        string(itemsJSON),
		ExpiresAt:    time.Now().UTC().Add(cartTTL),
	}

	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}, {Name: "barbershop_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"items", "expires_at", "updated_at"}),
		}).
		Create(&row).Error
}

func (s *PostgresStore) RemoveItem(ctx context.Context, key string, barbershopID uint, productID uint) (*domain.Cart, error) {
	cart, err := s.Get(ctx, key, barbershopID)
	if err != nil {
		return nil, err
	}
	if cart == nil {
		return nil, nil
	}

	filtered := make([]domain.Item, 0, len(cart.Items))
	for _, item := range cart.Items {
		if item.ProductID != productID {
			filtered = append(filtered, item)
		}
	}
	cart.Items = filtered
	cart.RecalculateTotals()

	if err := s.Save(ctx, cart); err != nil {
		return nil, err
	}

	return cart, nil
}

func (s *PostgresStore) Clear(ctx context.Context, key string, barbershopID uint) error {
	return s.db.WithContext(ctx).
		Where("key = ? AND barbershop_id = ?", key, barbershopID).
		Delete(&cartRow{}).Error
}

func rowToCart(row cartRow) (*domain.Cart, error) {
	var items []domain.Item
	if err := json.Unmarshal([]byte(row.Items), &items); err != nil {
		return nil, err
	}

	cart := &domain.Cart{
		Key:          row.Key,
		BarbershopID: row.BarbershopID,
		Items:        items,
	}
	cart.RecalculateTotals()

	return cart, nil
}
