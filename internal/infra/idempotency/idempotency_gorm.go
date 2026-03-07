package idempotency

import (
	"context"
	"errors"

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

// Save persists the idempotency key
func (s *GormStore) Save(
	ctx context.Context,
	key string,
) error {

	err := s.db.WithContext(ctx).
		Create(&IdempotencyKey{Key: key}).
		Error

	// Duplicate key = idempotent replay
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrDuplicateRequest
	}

	return err
}
