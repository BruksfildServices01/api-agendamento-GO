package models

import "time"

type User struct {
	ID           uint        `gorm:"primaryKey" json:"id"`
	BarbershopID *uint       `json:"barbershop_id"`
	Barbershop   *Barbershop `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	Name         string   `gorm:"size:100;not null"`
	Email        string   `gorm:"size:100;uniqueIndex;not null"`
	PasswordHash string   `gorm:"size:255;not null"`
	Phone        string   `gorm:"size:20"`
	Role         UserRole `gorm:"type:user_role;not null;default:'owner'"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
