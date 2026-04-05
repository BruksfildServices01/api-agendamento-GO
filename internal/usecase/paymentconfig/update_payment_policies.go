package paymentconfig

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/paymentconfig"
)

type UpdatePaymentPolicies struct {
	repo domain.Repository
}

func NewUpdatePaymentPolicies(
	repo domain.Repository,
) *UpdatePaymentPolicies {
	return &UpdatePaymentPolicies{repo: repo}
}

type UpdatePaymentPoliciesInput struct {
	DefaultRequirement   domain.PaymentRequirement      `json:"default_requirement"`
	PixExpirationMinutes *int                           `json:"pix_expiration_minutes,omitempty"`
	Categories           []domain.CategoryPaymentPolicy `json:"categories"`
}

func (uc *UpdatePaymentPolicies) Execute(
	ctx context.Context,
	barbershopID uint,
	in UpdatePaymentPoliciesInput,
) error {

	cfg, err := uc.repo.GetByBarbershopID(ctx, barbershopID)
	if err != nil {
		return err
	}

	// 1) Atualiza config global
	cfg.DefaultRequirement = in.DefaultRequirement
	if in.PixExpirationMinutes != nil {
		cfg.PixExpirationMinutes = *in.PixExpirationMinutes
	}

	// 2) Valida invariantes do config
	if err := domain.Validate(cfg); err != nil {
		return err
	}

	// 3) Persiste config global
	if err := uc.repo.UpsertConfig(ctx, cfg); err != nil {
		return err
	}

	// ======================================================
	// ✅ Opção B: PUT como fonte da verdade
	// ======================================================
	// 4) Remove todas as policies atuais do tenant
	if err := uc.repo.DeleteCategoryPolicies(ctx, barbershopID); err != nil {
		return err
	}

	// 5) Recria as policies vindas no payload
	if in.Categories == nil {
		in.Categories = []domain.CategoryPaymentPolicy{}
	}

	for _, p := range in.Categories {
		p.BarbershopID = barbershopID

		if err := p.Validate(); err != nil {
			return err
		}

		if err := uc.repo.UpsertCategoryPolicy(ctx, barbershopID, p); err != nil {
			return err
		}
	}

	return nil
}
