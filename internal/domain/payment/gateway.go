package payment

import (
	"context"
	"time"
)

// PaymentGateway é a interface genérica de integração com provedores de pagamento.
// Cada provider (Mercado Pago, PagBank, Stone/Pagar.me) deve implementar esta interface.
//
// As interfaces MPGateway e TransparentGateway são mantidas por compatibilidade
// e serão descontinuadas quando os use cases migrarem para esta interface.
type PaymentGateway interface {
	CreatePixPayment(ctx context.Context, input PixPaymentInput) (*PixPaymentResult, error)
	CreateCardPayment(ctx context.Context, input CardPaymentInput) (*CardPaymentResult, error)
	CreateHostedCheckout(ctx context.Context, input HostedCheckoutInput) (*HostedCheckoutResult, error)
	GetPaymentStatus(ctx context.Context, providerPaymentID string) (ProviderPaymentStatus, error)
}

// ProviderPaymentStatus representa os estados normalizados que qualquer provider pode retornar.
// A conversão de status específicos do provider é responsabilidade de cada adapter.
type ProviderPaymentStatus string

const (
	ProviderStatusPending   ProviderPaymentStatus = "pending"
	ProviderStatusApproved  ProviderPaymentStatus = "approved"
	ProviderStatusRejected  ProviderPaymentStatus = "rejected"
	ProviderStatusCancelled ProviderPaymentStatus = "cancelled"
	ProviderStatusInProcess ProviderPaymentStatus = "in_process"
)

// ── Inputs ────────────────────────────────────────────────────────────────────

type PixPaymentInput struct {
	AmountCents       int64
	Description       string
	ExternalReference string // nosso payment.ID — usado pelo webhook para reconciliar
	NotificationURL   string
	PayerEmail        string
	PayerCPF          string
}

// CardPaymentInput usa nomenclatura normalizada para CardBrand ("visa", "mastercard", "elo", "amex").
// Cada adapter traduz para o identificador interno do provider.
// CardToken é gerado pelo SDK do provider no frontend.
type CardPaymentInput struct {
	AmountCents       int64
	Description       string
	ExternalReference string
	NotificationURL   string
	PayerEmail        string
	PayerCPF          string
	CardToken         string
	CardBrand         string // "visa" | "mastercard" | "elo" | "amex" | "hipercard"
	Installments      int
	IsDebit           bool
}

type HostedCheckoutInput struct {
	AmountCents       int64
	Description       string
	ExternalReference string
	NotificationURL   string
	BackURLs          HostedCheckoutBackURLs
}

type HostedCheckoutBackURLs struct {
	Success string
	Pending string
	Failure string
}

// ── Results ───────────────────────────────────────────────────────────────────

// ProviderPaymentID é string para suportar IDs numéricos (MP) e UUIDs (PagBank, Stone).
type PixPaymentResult struct {
	ProviderPaymentID string
	Status            ProviderPaymentStatus
	QRCode            string
	QRCodeBase64      string
	ExpiresAt         *time.Time
}

type CardPaymentResult struct {
	ProviderPaymentID string
	Status            ProviderPaymentStatus
	StatusDetail      string
}

type HostedCheckoutResult struct {
	ProviderCheckoutID string
	RedirectURL        string
	SandboxURL         string
}
