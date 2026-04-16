package subscription

import "time"

type Subscription struct {
	ID                   uint
	BarbershopID         uint
	ClientID             uint
	PlanID               uint
	Status               Status
	CurrentPeriodStart   time.Time
	CurrentPeriodEnd     time.Time
	CutsUsedInPeriod     int
	CutsReservedInPeriod int

	Plan *Plan
}
