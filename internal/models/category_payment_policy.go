package models

import "time"

type CategoryPaymentPolicy struct {
	ID           uint `gorm:"primaryKey"`
	BarbershopID uint `gorm:"index;not null"`

	Category    ClientCategory     `gorm:"type:client_category;not null"`
	Requirement PaymentRequirement `gorm:"type:payment_requirement;not null"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
