package subscription

type Plan struct {
	ID                uint
	BarbershopID      uint
	Name              string
	MonthlyPriceCents int64
	CutsIncluded      int
	DiscountPercent   int
	Active            bool
}
