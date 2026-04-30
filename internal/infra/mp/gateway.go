package mp

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
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

// GetPaymentStatusByMPID consulta o status de um pagamento pelo ID numérico do MP.
// Mantido por compatibilidade com callers que ainda usam o ID int64 diretamente.
func (g *Gateway) GetPaymentStatusByMPID(mpPaymentID int64) (string, error) {
	resp, err := g.paymentClient.Get(context.Background(), int(mpPaymentID))
	if err != nil {
		return "", fmt.Errorf("mp get payment: %w", err)
	}
	return resp.Status, nil
}

// ── Implementação de domain.PaymentGateway ────────────────────────────────────
//
// Os métodos abaixo implementam a interface genérica PaymentGateway.
// Os métodos antigos (CreatePreference, CreatePayment, GetPaymentStatus) são mantidos
// para compatibilidade com os use cases existentes e serão removidos
// quando a migração para PaymentGateway estiver completa.

// CreatePixPayment implementa domain.PaymentGateway.
func (g *Gateway) CreatePixPayment(ctx context.Context, input domain.PixPaymentInput) (*domain.PixPaymentResult, error) {
	req := payment.Request{
		TransactionAmount: float64(input.AmountCents) / 100,
		Description:       input.Description,
		ExternalReference: input.ExternalReference,
		PaymentMethodID:   "pix",
		Installments:      1,
		Payer: &payment.PayerRequest{
			Email: input.PayerEmail,
		},
	}
	if isPublicURL(input.NotificationURL) {
		req.NotificationURL = input.NotificationURL
	}
	if input.PayerCPF != "" {
		req.Payer.Identification = &payment.IdentificationRequest{
			Type:   "CPF",
			Number: input.PayerCPF,
		}
	}

	resp, err := g.paymentClient.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("mp create pix: %w", err)
	}

	return &domain.PixPaymentResult{
		ProviderPaymentID: strconv.FormatInt(int64(resp.ID), 10),
		Status:            mapMPStatus(resp.Status),
		QRCode:            resp.PointOfInteraction.TransactionData.QRCode,
		QRCodeBase64:      resp.PointOfInteraction.TransactionData.QRCodeBase64,
	}, nil
}

// CreateCardPayment implementa domain.PaymentGateway.
// CardBrand ("visa", "mastercard", "elo", "amex") é traduzido para o payment_method_id do MP.
func (g *Gateway) CreateCardPayment(ctx context.Context, input domain.CardPaymentInput) (*domain.CardPaymentResult, error) {
	req := payment.Request{
		TransactionAmount: float64(input.AmountCents) / 100,
		Description:       input.Description,
		ExternalReference: input.ExternalReference,
		PaymentMethodID:   mpCardMethodID(input.CardBrand, input.IsDebit),
		Token:             input.CardToken,
		Installments:      input.Installments,
		Payer: &payment.PayerRequest{
			Email: input.PayerEmail,
		},
	}
	if isPublicURL(input.NotificationURL) {
		req.NotificationURL = input.NotificationURL
	}
	if input.PayerCPF != "" {
		req.Payer.Identification = &payment.IdentificationRequest{
			Type:   "CPF",
			Number: input.PayerCPF,
		}
	}

	resp, err := g.paymentClient.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("mp create card payment: %w", err)
	}

	return &domain.CardPaymentResult{
		ProviderPaymentID: strconv.FormatInt(int64(resp.ID), 10),
		Status:            mapMPStatus(resp.Status),
		StatusDetail:      resp.StatusDetail,
	}, nil
}

// CreateHostedCheckout implementa domain.PaymentGateway.
func (g *Gateway) CreateHostedCheckout(ctx context.Context, input domain.HostedCheckoutInput) (*domain.HostedCheckoutResult, error) {
	req := preference.Request{
		Items: []preference.ItemRequest{
			{
				Title:      input.Description,
				Quantity:   1,
				UnitPrice:  float64(input.AmountCents) / 100,
				CurrencyID: "BRL",
			},
		},
		BackURLs: &preference.BackURLsRequest{
			Success: input.BackURLs.Success,
			Pending: input.BackURLs.Pending,
			Failure: input.BackURLs.Failure,
		},
		AutoReturn:        "approved",
		ExternalReference: input.ExternalReference,
		NotificationURL:   input.NotificationURL,
	}

	resp, err := g.preferenceClient.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("mp create hosted checkout: %w", err)
	}

	return &domain.HostedCheckoutResult{
		ProviderCheckoutID: resp.ID,
		RedirectURL:        resp.InitPoint,
		SandboxURL:         resp.SandboxInitPoint,
	}, nil
}

// GetPaymentStatus implementa domain.PaymentGateway.
// providerPaymentID é o ID numérico do MP em formato string.
func (g *Gateway) GetPaymentStatus(ctx context.Context, providerPaymentID string) (domain.ProviderPaymentStatus, error) {
	id, err := strconv.ParseInt(providerPaymentID, 10, 64)
	if err != nil {
		return "", fmt.Errorf("mp: invalid payment id %q: %w", providerPaymentID, err)
	}
	resp, err := g.paymentClient.Get(ctx, int(id))
	if err != nil {
		return "", fmt.Errorf("mp get payment status: %w", err)
	}
	return mapMPStatus(resp.Status), nil
}

// mapMPStatus normaliza o status do MP para o ProviderPaymentStatus genérico.
func mapMPStatus(s string) domain.ProviderPaymentStatus {
	switch s {
	case "approved":
		return domain.ProviderStatusApproved
	case "rejected":
		return domain.ProviderStatusRejected
	case "cancelled":
		return domain.ProviderStatusCancelled
	case "in_process", "authorized":
		return domain.ProviderStatusInProcess
	default:
		return domain.ProviderStatusPending
	}
}

// mpCardMethodID traduz CardBrand + IsDebit para o payment_method_id do Mercado Pago.
func mpCardMethodID(brand string, isDebit bool) string {
	b := strings.ToLower(brand)
	if isDebit {
		switch b {
		case "visa":
			return "debvisa"
		case "mastercard", "master":
			return "debmaster"
		case "elo":
			return "debelo"
		default:
			return "deb" + b
		}
	}
	if b == "mastercard" {
		return "master"
	}
	return b
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
