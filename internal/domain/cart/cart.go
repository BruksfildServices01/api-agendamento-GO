package cart

type Cart struct {
	Key           string
	BarbershopID  uint
	Items         []Item
	SubtotalCents int64
	TotalCents    int64
}

func New(
	key string,
	barbershopID uint,
) *Cart {
	return &Cart{
		Key:           key,
		BarbershopID:  barbershopID,
		Items:         []Item{},
		SubtotalCents: 0,
		TotalCents:    0,
	}
}

func (c *Cart) RecalculateTotals() {
	var subtotal int64
	for _, item := range c.Items {
		subtotal += item.LineTotalCents
	}

	c.SubtotalCents = subtotal
	c.TotalCents = subtotal
}
