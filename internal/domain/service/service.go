package service

type Service struct {
	ID           uint
	BarbershopID uint
	Name         string
	Description  string
	DurationMin  int
	Price        int64
	Active       bool
	Category     string
}
