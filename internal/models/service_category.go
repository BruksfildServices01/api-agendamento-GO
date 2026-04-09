package models

import "time"

type ServiceCategory struct {
	ID           uint        `gorm:"primaryKey"`
	BarbershopID uint        `gorm:"index;not null"`
	Barbershop   *Barbershop `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Name         string      `gorm:"size:100;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
