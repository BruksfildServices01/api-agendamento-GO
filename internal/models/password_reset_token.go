package models

import "time"

type PasswordResetToken struct {
	ID        uint       `gorm:"primaryKey"`
	UserID    uint       `gorm:"not null"`
	Token     string     `gorm:"uniqueIndex;size:64;not null"`
	ExpiresAt time.Time  `gorm:"not null"`
	UsedAt    *time.Time
	CreatedAt time.Time
}
