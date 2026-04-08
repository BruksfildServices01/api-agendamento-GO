package paymentconfig

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/paymentconfig"
)

type GetPaymentPolicies struct {
	repo domain.Repository
}

func NewGetPaymentPolicies(
	repo domain.Repository,
) *GetPaymentPolicies {
	return &GetPaymentPolicies{repo: repo}
}

type PaymentPoliciesOutput struct {
	DefaultRequirement   domain.PaymentRequirement      `json:"default_requirement"`
	PixExpirationMinutes int                            `json:"pix_expiration_minutes"`
	Categories           []domain.CategoryPaymentPolicy `json:"categories"`
	MPPublicKey          string                         `json:"mp_public_key"`
	MPAccessTokenSet     bool                           `json:"mp_access_token_set"`
}

func (uc *GetPaymentPolicies) Execute(
	ctx context.Context,
	barbershopID uint,
) (*PaymentPoliciesOutput, error) {

	cfg, err := uc.repo.GetByBarbershopID(ctx, barbershopID)
	if err != nil {
		return nil, err
	}

	categories, err := uc.repo.ListCategoryPolicies(ctx, barbershopID)
	if err != nil {
		return nil, err
	}

	return &PaymentPoliciesOutput{
		DefaultRequirement:   cfg.DefaultRequirement,
		PixExpirationMinutes: cfg.PixExpirationMinutes,
		Categories:           categories,
		MPPublicKey:          cfg.MPPublicKey,
		MPAccessTokenSet:     cfg.MPAccessToken != "",
	}, nil
}
