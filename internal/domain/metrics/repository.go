package metrics

import "context"

type ClientMetricsRepository interface {
	GetOrCreate(
		ctx context.Context,
		barbershopID uint,
		clientID uint,
	) (*ClientMetrics, error)

	FindByBarbershop(
		ctx context.Context,
		barbershopID uint,
	) ([]*ClientMetrics, error)

	Save(ctx context.Context, m *ClientMetrics) error
}
