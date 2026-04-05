package models

import "time"

// AppointmentTicket holds the public token that lets a client
// view, cancel or reschedule their own appointment without login.
type AppointmentTicket struct {
	ID            uint      `gorm:"primaryKey"`
	AppointmentID uint      `gorm:"uniqueIndex;not null"`
	BarbershopID  uint      `gorm:"not null;index"`
	Token         string    `gorm:"size:64;uniqueIndex;not null"`
	ExpiresAt     time.Time `gorm:"not null"`
	CreatedAt     time.Time
}
