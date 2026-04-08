package models

import "time"

type BarbershopPaymentConfig struct {
	ID           uint `gorm:"primaryKey"`
	BarbershopID uint `gorm:"uniqueIndex;not null"`

	DefaultRequirement   PaymentRequirement `gorm:"type:payment_requirement;not null"`
	PixExpirationMinutes int                `gorm:"not null;default:4"`

	// Credenciais Mercado Pago por barbearia
	MPAccessToken string `gorm:"column:mp_access_token"`
	MPPublicKey   string `gorm:"column:mp_public_key"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
