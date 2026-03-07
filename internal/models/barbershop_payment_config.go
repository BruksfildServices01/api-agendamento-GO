package models

import "time"

type BarbershopPaymentConfig struct {
	ID           uint `gorm:"primaryKey"`
	BarbershopID uint `gorm:"uniqueIndex;not null"`

	DefaultRequirement   PaymentRequirement `gorm:"type:payment_requirement;not null"`
	PixExpirationMinutes int                `gorm:"not null;default:15"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
