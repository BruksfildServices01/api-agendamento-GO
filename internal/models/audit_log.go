package models

import "time"

type AuditLog struct {
	ID uint `gorm:"primaryKey"`

	BarbershopID uint
	UserID       *uint

	Action   string `gorm:"size:50;not null"`
	Entity   string `gorm:"size:50"`
	EntityID *uint
	Metadata string `gorm:"type:text"`

	CreatedAt time.Time
}
