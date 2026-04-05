package appointment

import "context"

type BarbershopInfo struct {
	ID       uint
	Timezone string
}

type BarbershopLister interface {
	ListBarbershops(ctx context.Context) ([]BarbershopInfo, error)
}
