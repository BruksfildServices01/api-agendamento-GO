package models

import "time"

//
// ======================================================
// ENUM VALUES
// ======================================================
//

const (
	AppointmentStatusScheduled       AppointmentStatus = "scheduled"
	AppointmentStatusAwaitingPayment AppointmentStatus = "awaiting_payment"
	AppointmentStatusCompleted       AppointmentStatus = "completed"
	AppointmentStatusCancelled       AppointmentStatus = "cancelled"
	AppointmentStatusNoShow          AppointmentStatus = "no_show"
)

const (
	CreatedByClient AppointmentCreatedBy = "client"
	CreatedByBarber AppointmentCreatedBy = "barber"
)

const (
	PaymentIntentPayLater PaymentIntentType = "pay_later"
	PaymentIntentPaid     PaymentIntentType = "paid"
)

const (
	NoShowSourceAuto   NoShowSourceType = "auto"
	NoShowSourceManual NoShowSourceType = "manual"
)

//
// ======================================================
// MODEL
// ======================================================
//

type Appointment struct {
	ID uint `gorm:"primaryKey"`

	BarbershopID *uint       `gorm:"index"`
	Barbershop   *Barbershop `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	BarberID *uint `gorm:"index"`
	Barber   *User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	ClientID *uint   `gorm:"index"`
	Client   *Client `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	BarberProductID *uint              `gorm:"index"`
	BarberProduct   *BarbershopService `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	StartTime time.Time `gorm:"type:timestamptz;not null"`
	EndTime   time.Time `gorm:"type:timestamptz;not null"`

	Status        AppointmentStatus    `gorm:"type:appointment_status;not null;default:'scheduled'"`
	CreatedBy     AppointmentCreatedBy `gorm:"type:appointment_created_by;not null;default:'client'"`
	PaymentIntent PaymentIntentType    `gorm:"type:payment_intent_type;not null;default:'pay_later'"`

	Notes string `gorm:"size:255"`

	CancelledAt  *time.Time
	CompletedAt  *time.Time
	NoShowAt     *time.Time
	NoShowSource *NoShowSourceType `gorm:"type:no_show_source_type"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
