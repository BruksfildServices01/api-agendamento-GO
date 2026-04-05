package models

import "time"

type OrderType string

const (
	OrderTypeProduct OrderType = "product"
)

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusPaid      OrderStatus = "paid"
	OrderStatusCancelled OrderStatus = "cancelled"
)

type Order struct {
	ID uint `gorm:"primaryKey"`

	BarbershopID uint        `gorm:"index;not null"`
	Barbershop   *Barbershop `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	ClientID *uint   `gorm:"index"`
	Client   *Client `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	Type   OrderType   `gorm:"type:order_type;not null"`
	Status OrderStatus `gorm:"type:order_status;not null;default:'pending'"`

	SubtotalAmount int64 `gorm:"type:bigint;not null;default:0"`
	DiscountAmount int64 `gorm:"type:bigint;not null;default:0"`
	TotalAmount    int64 `gorm:"type:bigint;not null;default:0"`

	Items []OrderItem `gorm:"foreignKey:OrderID"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
