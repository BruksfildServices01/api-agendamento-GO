package subscription

type Plan struct {
	ID                uint
	BarbershopID      uint
	Name              string
	MonthlyPriceCents int64
	DurationDays      int
	CutsIncluded      int
	DiscountPercent   int
	ServiceIDs        []uint
	Active            bool
	ActiveSubscribers int
}
