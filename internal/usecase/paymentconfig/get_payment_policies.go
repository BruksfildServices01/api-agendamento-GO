package paymentconfig

import (
	"context"

	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/paymentconfig"
)

type GetPaymentPolicies struct {
	repo domain.Repository
	db   *gorm.DB
}

func NewGetPaymentPolicies(
	repo domain.Repository,
	db *gorm.DB,
) *GetPaymentPolicies {
	return &GetPaymentPolicies{repo: repo, db: db}
}

type PaymentPoliciesOutput struct {
	DefaultRequirement   domain.PaymentRequirement      `json:"default_requirement"`
	PixExpirationMinutes int                            `json:"pix_expiration_minutes"`
	Categories           []domain.CategoryPaymentPolicy `json:"categories"`
	MPPublicKey          string                         `json:"mp_public_key"`
	MPAccessTokenSet     bool                           `json:"mp_access_token_set"`
	// ProviderConnected é true quando qualquer gateway está ativo
	// (MP legado OU PagBank via barbershop_payment_providers).
	// O frontend usa esse campo para decidir se pagamento online está disponível.
	ProviderConnected    bool                           `json:"provider_connected"`
	AcceptCash           bool                           `json:"accept_cash"`
	AcceptPix            bool                           `json:"accept_pix"`
	AcceptCredit         bool                           `json:"accept_credit"`
	AcceptDebit          bool                           `json:"accept_debit"`
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

	mpLegacyConnected := cfg.MPPublicKey != "" && cfg.MPAccessToken != ""

	var modernProviderCount int64
	if uc.db != nil {
		uc.db.WithContext(ctx).
			Table("barbershop_payment_providers").
			Where("barbershop_id = ? AND enabled = true AND credentials_encrypted IS NOT NULL", barbershopID).
			Count(&modernProviderCount)
	}

	return &PaymentPoliciesOutput{
		DefaultRequirement:   cfg.DefaultRequirement,
		PixExpirationMinutes: cfg.PixExpirationMinutes,
		Categories:           categories,
		MPPublicKey:          cfg.MPPublicKey,
		MPAccessTokenSet:     cfg.MPAccessToken != "",
		ProviderConnected:    mpLegacyConnected || modernProviderCount > 0,
		AcceptCash:           cfg.AcceptCash,
		AcceptPix:            cfg.AcceptPix,
		AcceptCredit:         cfg.AcceptCredit,
		AcceptDebit:          cfg.AcceptDebit,
	}, nil
}
