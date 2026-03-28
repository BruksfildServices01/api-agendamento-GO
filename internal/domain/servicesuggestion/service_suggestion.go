package servicesuggestion

type SuggestedProduct struct {
	ID            uint
	BarbershopID  uint
	Name          string
	Description   string
	Category      string
	Price         int64
	Stock         int
	Active        bool
	OnlineVisible bool
}

type ServiceSuggestion struct {
	ID           uint
	BarbershopID uint
	ServiceID    uint
	ProductID    uint
	Active       bool

	Product *SuggestedProduct
}
