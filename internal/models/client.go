package models

import "time"

type Client struct {
	ID           uint        `gorm:"primaryKey"`
	BarbershopID *uint       `gorm:"index"`
	Barbershop   *Barbershop `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	Name  string `gorm:"size:100;not null"`
	Phone string `gorm:"size:20"`
	Email string `gorm:"size:100"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
