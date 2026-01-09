package models

import "time"

type BarberProduct struct {
	ID           uint `gorm:"primaryKey" json:"id"`
	BarbershopID uint `json:"barbershop_id"`

	Name        string  `gorm:"size:100;not null" json:"name"`
	Description string  `gorm:"size:255" json:"description"`
	DurationMin int     `json:"duration_min"`
	Price       float64 `json:"price"`
	Active      bool    `gorm:"default:true" json:"active"`

	Category string `gorm:"size:50" json:"category"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
