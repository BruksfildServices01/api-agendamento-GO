package payment

// MPPreference contém os dados da preferência criada no Mercado Pago.
type MPPreference struct {
	PreferenceID string
	InitPoint    string // URL de checkout (produção)
	SandboxPoint string // URL de checkout (sandbox)
}

// MPBackURLs define as URLs de retorno para o Mercado Pago redirecionar o usuário.
type MPBackURLs struct {
	Success string
	Pending string
	Failure string
}

// MPGateway é a interface para criação de preferências de pagamento no Mercado Pago.
type MPGateway interface {
	CreatePreference(
		amountCents int64,
		description string,
		externalReference string,
		notificationURL string,
		backURLs MPBackURLs,
	) (*MPPreference, error)
}

// TransparentPaymentInput representa os dados para criar um pagamento no Checkout Transparente.
type TransparentPaymentInput struct {
	AmountCents       int64
	Description       string
	ExternalReference string
	NotificationURL   string
	PayerEmail        string
	PayerCPF          string // obrigatório para PIX
	PaymentMethodID   string // "pix", "visa", "master", "elo", "amex", "debelo"
	Token             string // token do cartão (vazio para PIX)
	Installments      int    // 1 para PIX e débito
}

// TransparentPaymentResult representa o resultado de um pagamento transparente.
type TransparentPaymentResult struct {
	MPPaymentID  int64
	Status       string // "pending", "approved", "rejected", "in_process"
	StatusDetail string
	// Campos PIX
	QRCode       string
	QRCodeBase64 string
	TicketURL    string
}

// TransparentGateway é a interface para criação de pagamentos via Checkout Transparente.
type TransparentGateway interface {
	CreatePayment(input TransparentPaymentInput) (*TransparentPaymentResult, error)
}
