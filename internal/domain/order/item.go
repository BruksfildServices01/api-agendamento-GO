package order

type OrderItem struct {
	ID       uint
	OrderID  uint
	ItemID   uint
	ItemName string

	Quantity  int
	UnitPrice int64
	Total     int64
}
