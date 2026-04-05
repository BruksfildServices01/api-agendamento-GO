package models

import "time"

type AppointmentClosure struct {
	ID uint `gorm:"primaryKey"`

	AppointmentID uint         `gorm:"uniqueIndex;not null"`
	Appointment   *Appointment `gorm:"constraint:OnDelete:CASCADE;"`

	BarbershopID uint        `gorm:"index;not null"`
	Barbershop   *Barbershop `gorm:"constraint:OnDelete:CASCADE;"`

	ServiceID   *uint
	ServiceName string `gorm:"size:150"`

	ReferenceAmountCents int64  `gorm:"type:bigint;not null"`
	FinalAmountCents     *int64 `gorm:"type:bigint"`

	SubscriptionConsumeStatus *string
	SubscriptionPlanID        *uint

	SubscriptionCovered    bool `gorm:"not null"`
	RequiresNormalCharging bool `gorm:"not null"`
	ConfirmNormalCharging  bool `gorm:"not null"`

	OperationalNote string `gorm:"size:255"`

	// Sprint 6: fechamento operacional real
	ActualServiceID   *uint  `gorm:"index"`
	ActualServiceName string `gorm:"size:150"`
	PaymentMethod     string `gorm:"size:20"` // cash|card|pix|subscription
	AdditionalOrderID *uint  `gorm:"index"`
	SuggestionRemoved bool   `gorm:"not null;default:false"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
