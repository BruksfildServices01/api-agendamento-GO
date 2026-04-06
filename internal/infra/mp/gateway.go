package mp

import (
	"context"
	"fmt"

	"github.com/mercadopago/sdk-go/pkg/config"
	"github.com/mercadopago/sdk-go/pkg/payment"
	"github.com/mercadopago/sdk-go/pkg/preference"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
)

// Gateway integra com as APIs do Mercado Pago (Checkout Pro e Checkout Transparente).
type Gateway struct {
	preferenceClient preference.Client
	paymentClient    payment.Client
}

// New cria o gateway MP com o access token fornecido.
func New(accessToken string) (*Gateway, error) {
	cfg, err := config.New(accessToken)
	if err != nil {
		return nil, fmt.Errorf("mp config: %w", err)
	}
	return &Gateway{
		preferenceClient: preference.NewClient(cfg),
		paymentClient:    payment.NewClient(cfg),
	}, nil
}

// CreatePreference cria uma preferência de pagamento no Mercado Pago (Checkout Pro).
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

	resp, err := g.preferenceClient.Create(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("mp create preference: %w", err)
	}

	return &domain.MPPreference{
		PreferenceID: resp.ID,
		InitPoint:    resp.InitPoint,
		SandboxPoint: resp.SandboxInitPoint,
	}, nil
}

// CreatePayment cria um pagamento via Checkout Transparente (PIX, cartão crédito/débito).
func (g *Gateway) CreatePayment(input domain.TransparentPaymentInput) (*domain.TransparentPaymentResult, error) {
	amountFloat := float64(input.AmountCents) / 100

	req := payment.Request{
		TransactionAmount: amountFloat,
		Description:       input.Description,
		ExternalReference: input.ExternalReference,
		NotificationURL:   input.NotificationURL,
		PaymentMethodID:   input.PaymentMethodID,
		Token:             input.Token,
		Installments:      input.Installments,
		Payer: &payment.PayerRequest{
			Email: input.PayerEmail,
			Identification: &payment.IdentificationRequest{
				Type:   "CPF",
				Number: input.PayerCPF,
			},
		},
	}

	resp, err := g.paymentClient.Create(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("mp create payment: %w", err)
	}

	return &domain.TransparentPaymentResult{
		MPPaymentID:  int64(resp.ID),
		Status:       resp.Status,
		StatusDetail: resp.StatusDetail,
		QRCode:       resp.PointOfInteraction.TransactionData.QRCode,
		QRCodeBase64: resp.PointOfInteraction.TransactionData.QRCodeBase64,
		TicketURL:    resp.PointOfInteraction.TransactionData.TicketURL,
	}, nil
}
