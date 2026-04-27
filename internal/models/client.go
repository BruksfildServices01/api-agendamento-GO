package models

import "time"

type Client struct {
	ID           uint        `gorm:"primaryKey"`
	BarbershopID *uint       `gorm:"index"`
	Barbershop   *Barbershop `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	Name  string `gorm:"size:100;not null"`
	Phone string `gorm:"size:20"`
	Email string `gorm:"size:100"`

	// LGPD: preenchidos quando dados pessoais são removidos a pedido do titular.
	// Após anonimização: Name="Cliente removido", Phone=NULL, Email=NULL.
	AnonymizedAt     *time.Time `gorm:"column:anonymized_at"`
	AnonymizedReason *string    `gorm:"size:50;column:anonymized_reason"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
