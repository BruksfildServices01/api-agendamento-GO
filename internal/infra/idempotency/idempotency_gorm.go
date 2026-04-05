package idempotency

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type IdempotencyKey struct {
	Key string `gorm:"primaryKey;size:128"`
}

// GormStore implements Store
type GormStore struct {
	db *gorm.DB
}

func NewGormStore(db *gorm.DB) *GormStore {
	return &GormStore{db: db}
}

// Exists checks if the idempotency key already exists
func (s *GormStore) Exists(
	ctx context.Context,
	key string,
) (bool, error) {

	var count int64
	err := s.db.WithContext(ctx).
		Model(&IdempotencyKey{}).
		Where("key = ?", key).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (s *GormStore) Save(
	ctx context.Context,
	key string,
) error {

	err := s.db.WithContext(ctx).
		Create(&IdempotencyKey{Key: key}).
		Error

	if err == nil {
		return nil
	}

	// Duplicate key = idempotent replay
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrDuplicateRequest
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicateRequest
	}

	if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
		return ErrDuplicateRequest
	}

	return err
}
