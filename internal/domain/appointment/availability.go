package appointment

import "time"

type AvailabilityInput struct {
	BarbershopID uint
	BarberID     uint
	ProductID    uint
	Date         time.Time
}

type TimeSlot struct {
	Start string `json:"start"`
	End   string `json:"end"`
}
