// Package pagbank integra com a API do PagBank via HTTP direto (sem SDK oficial Go).
//
// Documentação: https://developer.pagbank.com.br/
// Sandbox:      https://sandbox.api.pagseguro.com
// Produção:     https://api.pagseguro.com
package pagbank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
)

const (
	baseURLProd    = "https://api.pagseguro.com"
	baseURLSandbox = "https://sandbox.api.pagseguro.com"

	pixExpirationHours = 1 // PIX expira em 1 hora por padrão
)

var httpClient = &http.Client{Timeout: 20 * time.Second}

// Gateway integra com a API do PagBank para criação de pagamentos PIX e cartão.
// Implementa domain.TransparentGateway e domain.PaymentGateway.
type Gateway struct {
	accessToken string
	baseURL     string
}

// New cria um Gateway PagBank com o access token da barbearia.
// sandbox=true usa o ambiente de testes.
func New(accessToken string, sandbox bool) (*Gateway, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("pagbank: access token não pode ser vazio")
	}
	base := baseURLProd
	if sandbox {
		base = baseURLSandbox
	}
	return &Gateway{accessToken: accessToken, baseURL: base}, nil
}

// ── domain.PaymentGateway ─────────────────────────────────────────────────────

// CreatePixPayment implementa domain.PaymentGateway.
func (g *Gateway) CreatePixPayment(ctx context.Context, input domain.PixPaymentInput) (*domain.PixPaymentResult, error) {
	expiresAt := time.Now().UTC().Add(pixExpirationHours * time.Hour)

	req := pixOrderRequest{
		ReferenceID: input.ExternalReference,
		Customer: orderCustomer{
			Name:  input.PayerEmail, // PagBank exige nome — usando email como fallback se não tiver
			Email: input.PayerEmail,
			TaxID: strings.ReplaceAll(input.PayerCPF, ".", ""),
		},
		Items: []orderItem{
			{
				Name:       input.Description,
				Quantity:   1,
				UnitAmount: input.AmountCents,
			},
		},
		QRCodes: []orderQRCode{
			{
				Amount:         qrCodeAmount{Value: input.AmountCents},
				ExpirationDate: expiresAt.Format(time.RFC3339),
			},
		},
	}
	if input.NotificationURL != "" {
		req.NotificationURLs = []string{input.NotificationURL}
	}

	var resp orderResponse
	if err := g.post(ctx, "/orders", req, &resp); err != nil {
		return nil, fmt.Errorf("pagbank create pix: %w", err)
	}

	if len(resp.QRCodes) == 0 {
		return nil, fmt.Errorf("pagbank: resposta sem QR code")
	}

	qr := resp.QRCodes[0]
	result := &domain.PixPaymentResult{
		ProviderPaymentID: qr.ID, // QRC_XXXXX
		Status:            domain.ProviderStatusPending,
		QRCode:            qr.Text,
	}

	// Extrai base64 da imagem do QR code (rel=QRCODE, media=image/png com data:)
	for _, link := range qr.Links {
		if link.Rel == "QRCODE" && strings.HasPrefix(link.Href, "data:image/png;base64,") {
			result.QRCodeBase64 = strings.TrimPrefix(link.Href, "data:image/png;base64,")
			break
		}
	}

	t := expiresAt
	result.ExpiresAt = &t
	return result, nil
}

// CreateCardPayment implementa domain.PaymentGateway.
func (g *Gateway) CreateCardPayment(ctx context.Context, input domain.CardPaymentInput) (*domain.CardPaymentResult, error) {
	cardType := "CREDIT_CARD"
	if input.IsDebit {
		cardType = "DEBIT_CARD"
	}

	installments := input.Installments
	if installments <= 0 {
		installments = 1
	}

	req := cardOrderRequest{
		ReferenceID: input.ExternalReference,
		Customer: orderCustomer{
			Name:  input.PayerEmail,
			Email: input.PayerEmail,
			TaxID: strings.ReplaceAll(input.PayerCPF, ".", ""),
		},
		Items: []orderItem{
			{
				Name:       input.Description,
				Quantity:   1,
				UnitAmount: input.AmountCents,
			},
		},
		Charges: []orderCharge{
			{
				ReferenceID: input.ExternalReference,
				Amount:      chargeAmount{Value: input.AmountCents},
				PaymentMethod: paymentMethod{
					Type:         cardType,
					Installments: installments,
					Capture:      true,
					Card:         cardData{Encrypted: input.CardToken},
					Holder: cardHolder{
						Name:  input.PayerEmail,
						TaxID: strings.ReplaceAll(input.PayerCPF, ".", ""),
					},
				},
			},
		},
	}
	if input.NotificationURL != "" {
		req.NotificationURLs = []string{input.NotificationURL}
	}

	var resp orderResponse
	if err := g.post(ctx, "/orders", req, &resp); err != nil {
		return nil, fmt.Errorf("pagbank create card: %w", err)
	}

	if len(resp.Charges) == 0 {
		return nil, fmt.Errorf("pagbank: resposta sem charge")
	}

	charge := resp.Charges[0]
	return &domain.CardPaymentResult{
		ProviderPaymentID: charge.ID, // CHAR_XXXXX
		Status:            domain.ProviderPaymentStatus(mapStatus(charge.Status)),
		StatusDetail:      charge.PaymentResponse.Message,
	}, nil
}

// CreateHostedCheckout implementa domain.PaymentGateway.
// PagBank não tem checkout hosted equivalente ao MP Checkout Pro nesta integração.
func (g *Gateway) CreateHostedCheckout(_ context.Context, _ domain.HostedCheckoutInput) (*domain.HostedCheckoutResult, error) {
	return nil, fmt.Errorf("pagbank: hosted checkout não suportado — use PIX ou cartão transparente")
}

// GetPaymentStatus implementa domain.PaymentGateway.
// providerPaymentID pode ser ID de order (ORD_) ou charge (CHAR_).
func (g *Gateway) GetPaymentStatus(ctx context.Context, providerPaymentID string) (domain.ProviderPaymentStatus, error) {
	var resp orderResponse

	var path string
	if strings.HasPrefix(providerPaymentID, "CHAR_") {
		path = "/charges/" + providerPaymentID
		var chargeResp chargeResponse
		if err := g.get(ctx, path, &chargeResp); err != nil {
			return "", fmt.Errorf("pagbank get charge status: %w", err)
		}
		return domain.ProviderPaymentStatus(mapStatus(chargeResp.Status)), nil
	}

	// QRC_ ou ORD_: busca o pedido
	path = "/orders/" + providerPaymentID
	if err := g.get(ctx, path, &resp); err != nil {
		return "", fmt.Errorf("pagbank get order status: %w", err)
	}

	// Para orders com QR code, consulta o status via endpoint de charges.
	// A API do PagBank retorna o status de pagamento dentro das charges, não no QR code diretamente.
	if len(resp.Charges) == 0 && len(resp.QRCodes) > 0 {
		return domain.ProviderStatusPending, nil
	}
	if len(resp.Charges) > 0 {
		return domain.ProviderPaymentStatus(mapStatus(resp.Charges[0].Status)), nil
	}

	return domain.ProviderStatusPending, nil
}

// ── domain.TransparentGateway (interface antiga — compatibilidade) ─────────────

// CreatePayment implementa domain.TransparentGateway para compatibilidade com
// os use cases existentes que ainda usam a interface antiga.
// PaymentMethodID == "pix" → PIX; qualquer outro → cartão.
func (g *Gateway) CreatePayment(input domain.TransparentPaymentInput) (*domain.TransparentPaymentResult, error) {
	ctx := context.Background()

	if input.PaymentMethodID == "pix" {
		result, err := g.CreatePixPayment(ctx, domain.PixPaymentInput{
			AmountCents:       input.AmountCents,
			Description:       input.Description,
			ExternalReference: input.ExternalReference,
			NotificationURL:   input.NotificationURL,
			PayerEmail:        input.PayerEmail,
			PayerCPF:          input.PayerCPF,
		})
		if err != nil {
			return nil, err
		}
		return &domain.TransparentPaymentResult{
			ProviderPaymentID: result.ProviderPaymentID,
			Status:            "pending",
			QRCode:            result.QRCode,
			QRCodeBase64:      result.QRCodeBase64,
		}, nil
	}

	// Cartão
	result, err := g.CreateCardPayment(ctx, domain.CardPaymentInput{
		AmountCents:       input.AmountCents,
		Description:       input.Description,
		ExternalReference: input.ExternalReference,
		NotificationURL:   input.NotificationURL,
		PayerEmail:        input.PayerEmail,
		PayerCPF:          input.PayerCPF,
		CardToken:         input.Token,
		Installments:      input.Installments,
		IsDebit:           isPagBankDebit(input.PaymentMethodID),
	})
	if err != nil {
		return nil, err
	}
	return &domain.TransparentPaymentResult{
		ProviderPaymentID: result.ProviderPaymentID,
		Status:            string(result.Status),
		StatusDetail:      result.StatusDetail,
	}, nil
}

// WebhookPath retorna o path do webhook de notificação do PagBank.
// Permite que o use case construa a notification_url correta por provider,
// sem expor o provider name na interface TransparentGateway.
func (g *Gateway) WebhookPath() string {
	return "/api/webhooks/pagbank"
}

// ProviderName retorna o identificador do provider gravado em payments.provider.
// Usado para associar um payment ao seu gateway de origem e permitir polling correto.
func (g *Gateway) ProviderName() string {
	return "pagbank"
}

// isPagBankDebit retorna true para métodos de débito PagBank.
func isPagBankDebit(method string) bool {
	return strings.HasPrefix(strings.ToLower(method), "deb")
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func (g *Gateway) post(ctx context.Context, path string, body, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.accessToken)

	return g.do(req, out)
}

func (g *Gateway) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+g.accessToken)
	return g.do(req, out)
}

func (g *Gateway) do(req *http.Request, out any) error {
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pagbank http: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("pagbank read body: %w", err)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("pagbank api error %d: %s", resp.StatusCode, string(data))
	}

	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("pagbank unmarshal: %w", err)
		}
	}
	return nil
}
