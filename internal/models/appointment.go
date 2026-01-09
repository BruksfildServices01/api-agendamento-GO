package models

import "time"

type Appointment struct {
	ID uint `gorm:"primaryKey" json:"id"`

	BarbershopID uint       `json:"barbershop_id"`
	Barbershop   Barbershop `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"barbershop"`

	BarberID uint `json:"barber_id"`
	Barber   User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"barber"`

	ClientID uint   `json:"client_id"`
	Client   Client `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"client"`

	BarberProductID uint          `json:"barber_product_id"`
	BarberProduct   BarberProduct `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"barber_product"`

	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`

	Status string `gorm:"size:20;default:'scheduled'" json:"status"`

	Notes       string     `gorm:"size:255" json:"notes"`
	CancelledAt *time.Time `json:"cancelled_at"`
	CompletedAt *time.Time `json:"completed_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
