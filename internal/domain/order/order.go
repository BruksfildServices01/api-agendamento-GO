package order

import "time"

type OrderType string

const (
	OrderTypeProduct OrderType = "product"
	OrderTypeMixed   OrderType = "mixed"
)

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusPaid      OrderStatus = "paid"
	OrderStatusCancelled OrderStatus = "cancelled"
)

type Order struct {
	ID           uint
	BarbershopID uint

	Type   OrderType
	Status OrderStatus

	TotalAmount int64

	Items []OrderItem

	CreatedAt time.Time
	UpdatedAt time.Time
}

func New(
	barbershopID uint,
	orderType OrderType,
) *Order {
	return &Order{
		BarbershopID: barbershopID,
		Type:         orderType,
		Status:       OrderStatusPending,
		Items:        []OrderItem{},
	}
}

func (o *Order) AddItem(
	itemID uint,
	name string,
	quantity int,
	unitPrice int64,
) error {

	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	if unitPrice < 0 {
		return ErrInvalidPrice
	}

	total := int64(quantity) * unitPrice

	o.Items = append(o.Items, OrderItem{
		ItemID:    itemID,
		ItemName:  name,
		Quantity:  quantity,
		UnitPrice: unitPrice,
		Total:     total,
	})

	o.recalculateTotal()
	return nil
}

func (o *Order) recalculateTotal() {
	var total int64

	for _, item := range o.Items {
		total += item.Total
	}

	o.TotalAmount = total
}

func (o *Order) Validate() error {
	if len(o.Items) == 0 {
		return ErrEmptyOrder
	}

	if o.TotalAmount <= 0 {
		return ErrInvalidTotal
	}

	return nil
}
