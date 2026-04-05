package cart

type Item struct {
	ProductID      uint
	ProductName    string
	Quantity       int
	UnitPriceCents int64
	LineTotalCents int64
}

func NewItem(
	productID uint,
	productName string,
	quantity int,
	unitPriceCents int64,
) Item {
	return Item{
		ProductID:      productID,
		ProductName:    productName,
		Quantity:       quantity,
		UnitPriceCents: unitPriceCents,
		LineTotalCents: int64(quantity) * unitPriceCents,
	}
}
