package mp

import (
	"context"
	"fmt"

	"github.com/mercadopago/sdk-go/pkg/config"
	"github.com/mercadopago/sdk-go/pkg/preference"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
)

// Gateway integra com a API de preferências do Mercado Pago (Checkout Pro).
// Documentação: https://www.mercadopago.com.br/developers/pt/reference/preferences/_checkout_preferences/post
type Gateway struct {
	client preference.Client
}

// New cria o gateway MP com o access token fornecido.
func New(accessToken string) (*Gateway, error) {
	cfg, err := config.New(accessToken)
	if err != nil {
		return nil, fmt.Errorf("mp config: %w", err)
	}
	return &Gateway{
		client: preference.NewClient(cfg),
	}, nil
}

// CreatePreference cria uma preferência de pagamento no Mercado Pago
// e retorna os dados necessários para iniciar o checkout.
func (g *Gateway) CreatePreference(
	amountCents int64,
	description string,
	externalReference string,
	notificationURL string,
	backURLs domain.MPBackURLs,
) (*domain.MPPreference, error) {

	amountFloat := float64(amountCents) / 100

	req := preference.Request{
		Items: []preference.ItemRequest{
			{
				Title:      description,
				Quantity:   1,
				UnitPrice:  amountFloat,
				CurrencyID: "BRL",
			},
		},
		BackURLs: &preference.BackURLsRequest{
			Success: backURLs.Success,
			Pending: backURLs.Pending,
			Failure: backURLs.Failure,
		},
		AutoReturn:        "approved",
		ExternalReference: externalReference,
		NotificationURL:   notificationURL,
	}

	resp, err := g.client.Create(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("mp create preference: %w", err)
	}

	return &domain.MPPreference{
		PreferenceID: resp.ID,
		InitPoint:    resp.InitPoint,
		SandboxPoint: resp.SandboxInitPoint,
	}, nil
}
