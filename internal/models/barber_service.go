package models

import "time"

type BarbershopService struct {
	ID           uint        `gorm:"primaryKey"`
	BarbershopID uint        `gorm:"index;not null"`
	Barbershop   *Barbershop `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	Name        string `gorm:"size:100;not null"`
	Description string `gorm:"size:255"`
	DurationMin int
	Price       int64  `gorm:"type:bigint;not null"`
	Active      bool   `gorm:"default:true"`
	Category    string `gorm:"size:50"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
