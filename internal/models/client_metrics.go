package models

import "time"

type ClientMetrics struct {
	ClientID     uint `gorm:"primaryKey"`
	BarbershopID uint `gorm:"primaryKey"`

	TotalAppointments     int `gorm:"not null;default:0"`
	CompletedAppointments int `gorm:"not null;default:0"`
	CancelledAppointments int `gorm:"not null;default:0"`
	NoShowAppointments    int `gorm:"not null;default:0"`

	RescheduledAppointments     int `gorm:"not null;default:0"`
	LateCancelledAppointments   int `gorm:"not null;default:0"`
	LateRescheduledAppointments int `gorm:"not null;default:0"`

	TotalSpent int64 `gorm:"type:bigint;not null;default:0"`

	FirstAppointmentAt *time.Time
	LastAppointmentAt  *time.Time
	LastCompletedAt    *time.Time
	LastCanceledAt     *time.Time

	LastNoShowAt          *time.Time
	LastLateCanceledAt    *time.Time
	LastLateRescheduledAt *time.Time

	Category       ClientCategory     `gorm:"type:client_category;not null;default:'new'"`
	CategorySource CategorySourceType `gorm:"type:category_source_type;not null;default:'auto'"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
