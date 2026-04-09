package service

type ServiceImage struct {
	ID       uint
	URL      string
	Position int
}

type Service struct {
	ID           uint
	BarbershopID uint
	Name         string
	Description  string
	DurationMin  int
	Price        int64
	Active       bool
	Category     string
	CategoryID   *uint
	Images       []ServiceImage
}
