package order

type OrderItem struct {
	ID      uint
	OrderID uint

	ProductID           uint
	ProductNameSnapshot string

	Quantity  int
	UnitPrice int64
	LineTotal int64
}
