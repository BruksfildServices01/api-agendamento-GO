package product

import "context"

type Product struct {
	ID           uint
	BarbershopID uint

	Name        string
	Description string
	Category    string
	Price       int64
	ImageURL    string

	Stock         int
	Active        bool
	OnlineVisible bool
}

type Repository interface {
	Create(ctx context.Context, p *Product) error
	Update(ctx context.Context, p *Product) error
	Delete(ctx context.Context, barbershopID uint, id uint) error
	GetByID(ctx context.Context, barbershopID uint, id uint) (*Product, error)
	ListByBarbershop(ctx context.Context, barbershopID uint) ([]*Product, error)

	ListPublicProducts(
		ctx context.Context,
		barbershopID uint,
		category string,
		query string,
	) ([]*Product, error)
}
