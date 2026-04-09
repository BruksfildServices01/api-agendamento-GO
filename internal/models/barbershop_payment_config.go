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

	// Formas de pagamento aceitas (presencialmente)
	AcceptCash   bool `gorm:"column:accept_cash;not null;default:true"`
	AcceptPix    bool `gorm:"column:accept_pix;not null;default:true"`
	AcceptCredit bool `gorm:"column:accept_credit;not null;default:true"`
	AcceptDebit  bool `gorm:"column:accept_debit;not null;default:true"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
