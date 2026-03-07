package models

import "time"

type BarbershopService struct {
	ID           uint        `gorm:"primaryKey"`
	BarbershopID *uint       `gorm:"index"`
	Barbershop   *Barbershop `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	Name        string `gorm:"size:100;not null"`
	Description string `gorm:"size:255"`
	DurationMin int
	Price       int64  `gorm:"type:bigint;not null"`
	Active      bool   `gorm:"default:true"`
	Category    string `gorm:"size:50"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
