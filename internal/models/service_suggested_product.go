package models

import "time"

type ServiceSuggestedProduct struct {
	ID uint `gorm:"primaryKey"`

	BarbershopID uint        `gorm:"index;not null"`
	Barbershop   *Barbershop `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	ServiceID uint               `gorm:"index;not null"`
	Service   *BarbershopService `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	ProductID uint     `gorm:"index;not null"`
	Product   *Product `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	Active bool `gorm:"not null;default:true"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
