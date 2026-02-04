package models

import "time"

type Payment struct {
	ID uint `gorm:"primaryKey" json:"id"`

	BarbershopID uint `json:"barbershop_id"`

	AppointmentID uint    `gorm:"uniqueIndex" json:"appointment_id"`
	TxID          *string `gorm:"column:txid;uniqueIndex;size:64" json:"txid"` // ⭐ FIX

	Amount float64 `json:"amount"`

	Status string `gorm:"size:20;not null" json:"status"`

	PaidAt    *time.Time `json:"paid_at"`
	ExpiresAt *time.Time `gorm:"column:expires_at" json:"expires_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
