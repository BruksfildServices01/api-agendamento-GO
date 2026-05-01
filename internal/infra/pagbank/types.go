package pagbank

// ── Requests ──────────────────────────────────────────────────────────────────

type orderCustomer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	TaxID string `json:"tax_id"` // CPF
}

type orderItem struct {
	Name       string `json:"name"`
	Quantity   int    `json:"quantity"`
	UnitAmount int64  `json:"unit_amount"` // centavos
}

type qrCodeAmount struct {
	Value int64 `json:"value"` // centavos
}

type orderQRCode struct {
	Amount         qrCodeAmount `json:"amount"`
	ExpirationDate string       `json:"expiration_date,omitempty"` // RFC3339
}

type chargeAmount struct {
	Value int64 `json:"value"`
}

type cardData struct {
	Encrypted string `json:"encrypted"` // token do SDK JS do PagBank
}

type cardHolder struct {
	Name  string `json:"name"`
	TaxID string `json:"tax_id"`
}

type paymentMethod struct {
	Type         string     `json:"type"`                   // CREDIT_CARD | DEBIT_CARD
	Installments int        `json:"installments"`
	Capture      bool       `json:"capture"`
	Card         cardData   `json:"card"`
	Holder       cardHolder `json:"holder"`
}

type orderCharge struct {
	ReferenceID   string        `json:"reference_id"`
	Amount        chargeAmount  `json:"amount"`
	PaymentMethod paymentMethod `json:"payment_method"`
}

// pixOrderRequest cria um pedido com QR code PIX.
type pixOrderRequest struct {
	ReferenceID      string         `json:"reference_id"`
	Customer         orderCustomer  `json:"customer"`
	Items            []orderItem    `json:"items"`
	QRCodes          []orderQRCode  `json:"qr_codes"`
	NotificationURLs []string       `json:"notification_urls,omitempty"`
}

// cardOrderRequest cria um pedido com pagamento por cartão.
type cardOrderRequest struct {
	ReferenceID      string        `json:"reference_id"`
	Customer         orderCustomer `json:"customer"`
	Items            []orderItem   `json:"items"`
	Charges          []orderCharge `json:"charges"`
	NotificationURLs []string      `json:"notification_urls,omitempty"`
}

// ── Responses ──────────────────────────────────────────────────────────────────

type qrCodeLink struct {
	Rel   string `json:"rel"`
	Href  string `json:"href"`
	Media string `json:"media"`
}

type qrCodeResponse struct {
	ID             string       `json:"id"`
	Text           string       `json:"text"`           // copia e cola EMV
	ExpirationDate string       `json:"expiration_date"`
	Links          []qrCodeLink `json:"links"`
}

type chargePaymentResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type chargeResponse struct {
	ID              string                `json:"id"`
	ReferenceID     string                `json:"reference_id"`
	Status          string                `json:"status"` // PAID | WAITING | DECLINED | CANCELED | IN_ANALYSIS
	PaymentResponse chargePaymentResponse `json:"payment_response"`
}

// orderResponse é a resposta de criação de pedido (PIX ou cartão).
type orderResponse struct {
	ID          string           `json:"id"`
	ReferenceID string           `json:"reference_id"`
	QRCodes     []qrCodeResponse `json:"qr_codes,omitempty"`
	Charges     []chargeResponse `json:"charges,omitempty"`
}

// ── Webhook ────────────────────────────────────────────────────────────────────

// WebhookPayload é o payload enviado pelo PagBank nos webhooks.
// O campo Order contém o pedido completo com status atualizado.
type WebhookPayload struct {
	Order webhookOrder `json:"order"`
}

type webhookOrder struct {
	ID          string                `json:"id"`
	ReferenceID string                `json:"reference_id"`
	QRCodes     []webhookQRCode       `json:"qr_codes,omitempty"`
	Charges     []webhookCharge       `json:"charges,omitempty"`
}

type webhookQRCode struct {
	ID     string `json:"id"`
	Status string `json:"status"` // PAID | WAITING | EXPIRED
}

type webhookCharge struct {
	ID          string `json:"id"`
	ReferenceID string `json:"reference_id"`
	Status      string `json:"status"` // PAID | WAITING | DECLINED | CANCELED | IN_ANALYSIS
}

// ── Status mapping ────────────────────────────────────────────────────────────

// mapStatus converte status PagBank para o vocabulário interno do sistema.
func mapStatus(s string) string {
	switch s {
	case "PAID":
		return "approved"
	case "DECLINED", "CANCELED":
		return "rejected"
	case "IN_ANALYSIS":
		return "in_process"
	default: // WAITING, etc.
		return "pending"
	}
}
