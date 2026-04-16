package mp

import (
	"context"
	"fmt"
	"net/url"
	"strings"

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
		PaymentMethodID:   input.PaymentMethodID,
		Token:             input.Token,
		Installments:      input.Installments,
		Payer: &payment.PayerRequest{
			Email: input.PayerEmail,
		},
	}

	// Só envia NotificationURL se for uma URL pública (não localhost).
	if isPublicURL(input.NotificationURL) {
		req.NotificationURL = input.NotificationURL
	}

	// Só envia Identification se o CPF foi fornecido — MP rejeita número vazio.
	if input.PayerCPF != "" {
		req.Payer.Identification = &payment.IdentificationRequest{
			Type:   "CPF",
			Number: input.PayerCPF,
		}
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

// GetPaymentStatus consulta o status de um pagamento pelo ID do MP.
func (g *Gateway) GetPaymentStatus(mpPaymentID int64) (string, error) {
	resp, err := g.paymentClient.Get(context.Background(), int(mpPaymentID))
	if err != nil {
		return "", fmt.Errorf("mp get payment: %w", err)
	}
	return resp.Status, nil
}

// isPublicURL retorna true apenas para URLs HTTPS não-localhost.
// MP rejeita notification_url que não seja pública e acessível.
func isPublicURL(raw string) bool {
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return false
	}
	return u.Scheme == "https"
}
