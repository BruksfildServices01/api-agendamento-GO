package models

import "time"

type User struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	BarbershopID uint       `json:"barbershop_id"`
	Barbershop   Barbershop `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"barbershop"`

	Name         string `gorm:"size:100;not null" json:"name"`
	Email        string `gorm:"size:100;uniqueIndex;not null" json:"email"`
	PasswordHash string `gorm:"size:255;not null" json:"-"`
	Phone        string `gorm:"size:20" json:"phone"`
	Role         string `gorm:"size:20;default:'owner'" json:"role"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
