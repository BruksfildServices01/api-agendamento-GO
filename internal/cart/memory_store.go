package cart

import (
	"context"
	"sync"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/cart"
)

type MemoryStore struct {
	mu    sync.RWMutex
	carts map[string]*domain.Cart
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		carts: make(map[string]*domain.Cart),
	}
}

func (s *MemoryStore) Get(
	_ context.Context,
	key string,
	barbershopID uint,
) (*domain.Cart, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cart, ok := s.carts[key]
	if !ok {
		return nil, nil
	}

	if cart.BarbershopID != barbershopID {
		return nil, nil
	}

	return cloneCart(cart), nil
}

func (s *MemoryStore) Save(
	_ context.Context,
	cart *domain.Cart,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.carts[cart.Key] = cloneCart(cart)
	return nil
}

func (s *MemoryStore) RemoveItem(
	_ context.Context,
	key string,
	barbershopID uint,
	productID uint,
) (*domain.Cart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart, ok := s.carts[key]
	if !ok || cart.BarbershopID != barbershopID {
		return nil, nil
	}

	filtered := make([]domain.Item, 0, len(cart.Items))
	for _, item := range cart.Items {
		if item.ProductID == productID {
			continue
		}
		filtered = append(filtered, item)
	}

	cart.Items = filtered
	cart.RecalculateTotals()
	s.carts[key] = cloneCart(cart)

	return cloneCart(cart), nil
}

func (s *MemoryStore) Clear(
	_ context.Context,
	key string,
	barbershopID uint,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart, ok := s.carts[key]
	if !ok {
		return nil
	}
	if cart.BarbershopID != barbershopID {
		return nil
	}

	delete(s.carts, key)
	return nil
}

func cloneCart(src *domain.Cart) *domain.Cart {
	if src == nil {
		return nil
	}

	items := make([]domain.Item, len(src.Items))
	copy(items, src.Items)

	return &domain.Cart{
		Key:           src.Key,
		BarbershopID:  src.BarbershopID,
		Items:         items,
		SubtotalCents: src.SubtotalCents,
		TotalCents:    src.TotalCents,
	}
}
