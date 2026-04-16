package subscription

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
)

type ReserveSubscriptionCut struct {
	repo domain.Repository
}

func NewReserveSubscriptionCut(repo domain.Repository) *ReserveSubscriptionCut {
	return &ReserveSubscriptionCut{repo: repo}
}

func (uc *ReserveSubscriptionCut) Execute(ctx context.Context, barbershopID, clientID uint) error {
	return uc.repo.ReserveSubscriptionCut(ctx, barbershopID, clientID)
}

type ReleaseSubscriptionCut struct {
	repo domain.Repository
}

func NewReleaseSubscriptionCut(repo domain.Repository) *ReleaseSubscriptionCut {
	return &ReleaseSubscriptionCut{repo: repo}
}

func (uc *ReleaseSubscriptionCut) Execute(ctx context.Context, barbershopID, clientID uint) error {
	return uc.repo.ReleaseSubscriptionCut(ctx, barbershopID, clientID)
}
