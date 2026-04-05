package models

import "time"

type Plan struct {
	ID                uint `gorm:"primaryKey"`
	BarbershopID      uint
	Name              string
	MonthlyPriceCents int64
	DurationDays      int
	CutsIncluded      int
	DiscountPercent   int
	Active            bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Subscription struct {
	ID                 uint `gorm:"primaryKey"`
	BarbershopID       uint
	ClientID           uint
	PlanID             uint
	Status             string
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
	CutsUsedInPeriod   int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
