package order

import "time"

type OrderType string

const (
	OrderTypeProduct OrderType = "product"
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
	ClientID     *uint

	Type   OrderType
	Status OrderStatus

	SubtotalAmount int64
	DiscountAmount int64
	TotalAmount    int64

	Items []OrderItem

	CreatedAt time.Time
	UpdatedAt time.Time
}

func New(
	barbershopID uint,
	clientID *uint,
) *Order {
	return &Order{
		BarbershopID:   barbershopID,
		ClientID:       clientID,
		Type:           OrderTypeProduct,
		Status:         OrderStatusPending,
		SubtotalAmount: 0,
		DiscountAmount: 0,
		TotalAmount:    0,
		Items:          []OrderItem{},
	}
}

func (o *Order) AddItem(
	productID uint,
	productName string,
	quantity int,
	unitPrice int64,
) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	if unitPrice < 0 {
		return ErrInvalidPrice
	}

	lineTotal := int64(quantity) * unitPrice

	o.Items = append(o.Items, OrderItem{
		ProductID:           productID,
		ProductNameSnapshot: productName,
		Quantity:            quantity,
		UnitPrice:           unitPrice,
		LineTotal:           lineTotal,
	})

	o.recalculateTotals()
	return nil
}

func (o *Order) recalculateTotals() {
	var subtotal int64

	for _, item := range o.Items {
		subtotal += item.LineTotal
	}

	o.SubtotalAmount = subtotal
	o.TotalAmount = o.SubtotalAmount - o.DiscountAmount
}

func (o *Order) Validate() error {
	if len(o.Items) == 0 {
		return ErrEmptyOrder
	}

	if o.SubtotalAmount <= 0 {
		return ErrInvalidSubtotal
	}

	if o.DiscountAmount < 0 {
		return ErrInvalidDiscount
	}

	if o.TotalAmount <= 0 {
		return ErrInvalidTotal
	}

	return nil
}
