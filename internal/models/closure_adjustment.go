package models

import "time"

// ClosureAdjustment records a post-closure correction.
// It never modifies the original AppointmentClosure — it stores the delta.
type ClosureAdjustment struct {
	ID           uint `gorm:"primaryKey"`
	ClosureID    uint `gorm:"not null;index"`
	BarbershopID uint `gorm:"not null;index"`
	BarberID     *uint

	// Delta fields — nil means "unchanged".
	DeltaFinalAmountCents *int64  `gorm:"type:bigint"`
	DeltaPaymentMethod    *string `gorm:"size:20"`
	DeltaOperationalNote  *string `gorm:"size:255"`

	Reason     string    `gorm:"size:255;not null"`
	AdjustedAt time.Time `gorm:"not null"`

	CreatedAt time.Time
}
