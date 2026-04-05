package service

import "context"

type Repository interface {
	Create(
		ctx context.Context,
		s *Service,
	) error

	Update(
		ctx context.Context,
		s *Service,
	) error

	GetByID(
		ctx context.Context,
		barbershopID uint,
		id uint,
	) (*Service, error)

	ListByBarbershop(
		ctx context.Context,
		barbershopID uint,
	) ([]*Service, error)

	ListActiveByBarbershop(
		ctx context.Context,
		barbershopID uint,
	) ([]*Service, error)

	ListPublicServices(
		ctx context.Context,
		barbershopID uint,
		category string,
		query string,
	) ([]*Service, error)
}
