package models

import "time"

// Cliente simples, sem login, vinculado à barbearia
type Client struct {
	ID           uint `gorm:"primaryKey" json:"id"`
	BarbershopID uint `json:"barbershop_id"`

	Name  string `gorm:"size:100;not null" json:"name"`
	Phone string `gorm:"size:20" json:"phone"`
	Email string `gorm:"size:100" json:"email"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
