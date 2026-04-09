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
	// BundledOrderID: quando um pagamento de agendamento também cobre um pedido de produtos,
	// o ID do pedido é armazenado aqui (sem violar a constraint payment_exactly_one_target).
	BundledOrderID *uint         `gorm:"column:bundled_order_id;index"`
	TxID           *string       `gorm:"column:txid;size:100;uniqueIndex"`
	QRCode        *string       `gorm:"type:text"`
	Amount        int64         `gorm:"type:bigint;not null"`
	Status        PaymentStatus `gorm:"type:payment_status;not null"`
	PaidAt        *time.Time
	ExpiresAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
