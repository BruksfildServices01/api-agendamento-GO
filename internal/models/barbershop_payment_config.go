package models

import "time"

type BarbershopPaymentConfig struct {
	ID uint `gorm:"primaryKey"`

	BarbershopID uint `gorm:"uniqueIndex;not null"`

	RequirePixOnBooking bool `gorm:"not null;default:false"`

	PixExpirationMinutes int `gorm:"not null;default:15"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
