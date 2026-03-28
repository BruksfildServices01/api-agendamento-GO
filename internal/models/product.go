package models

import "time"

type Product struct {
	ID uint `gorm:"primaryKey"`

	BarbershopID uint        `gorm:"index;not null"`
	Barbershop   *Barbershop `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	Name          string `gorm:"size:100;not null"`
	Description   string `gorm:"size:255"`
	Category      string `gorm:"size:50"`
	Price         int64  `gorm:"type:bigint;not null"`
	Stock         int    `gorm:"not null;default:0"`
	Active        bool   `gorm:"default:true"`
	OnlineVisible bool   `gorm:"not null;default:false"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
