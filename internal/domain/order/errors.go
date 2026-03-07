package order

import "errors"

var (
	ErrInvalidQuantity = errors.New("invalid quantity")
	ErrInvalidPrice    = errors.New("invalid unit price")
	ErrEmptyOrder      = errors.New("order has no items")
	ErrInvalidTotal    = errors.New("invalid total amount")
	ErrInvalidStatus   = errors.New("invalid status transition")
)
