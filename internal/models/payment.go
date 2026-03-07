package models

import "time"

type Payment struct {
	ID            uint          `gorm:"primaryKey"`
	BarbershopID  uint          `gorm:"index;not null"`
	Barbershop    *Barbershop   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	AppointmentID *uint         `gorm:"index"`
	Appointment   *Appointment  `gorm:"constraint:OnDelete:CASCADE;"`
	OrderID       *uint         `gorm:"index"`
	Order         *Order        `gorm:"constraint:OnDelete:CASCADE;"`
	TxID          *string       `gorm:"column:txid;size:100;uniqueIndex"`
	QRCode        *string       `gorm:"type:text"`
	Amount        int64         `gorm:"type:bigint;not null"`
	Status        PaymentStatus `gorm:"type:payment_status;not null"`
	PaidAt        *time.Time
	ExpiresAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
