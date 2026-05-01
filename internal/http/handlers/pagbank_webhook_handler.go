package handlers

import (
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/integration/payment/pagbank"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

// PagBankWebhookHandler processa notificações de pagamento do PagBank.
// Valida a assinatura RSA-SHA256 e delega a confirmação ao use case existente.
type PagBankWebhookHandler struct {
	db                  *gorm.DB
	markAsPaid          *ucPayment.MarkMPPaymentAsPaid
	sandbox             bool
}

func NewPagBankWebhookHandler(
	db *gorm.DB,
	markAsPaid *ucPayment.MarkMPPaymentAsPaid,
	sandbox bool,
) *PagBankWebhookHandler {
	return &PagBankWebhookHandler{
		db:         db,
		markAsPaid: markAsPaid,
		sandbox:    sandbox,
	}
}

// Handle processa POST /api/webhooks/pagbank
func (h *PagBankWebhookHandler) Handle(c *gin.Context) {
	// Lê o body para validação de assinatura e parsing.
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<16)) // 64KB
	if err != nil {
		c.Status(http.StatusOK) // sempre responde 200 para evitar reentradas
		return
	}

	signature := c.GetHeader("x-payload-signature")

	// Valida assinatura RSA-SHA256.
	// Em sandbox, signature é vazia — ValidateWebhookSignature aceita isso.
	// Precisamos de um access_token de qualquer barbearia PagBank para buscar a public key.
	accessToken := h.getAnyPagBankToken(c)
	if err := pagbank.ValidateWebhookSignature(c.Request.Context(), accessToken, signature, body, h.sandbox); err != nil {
		log.Printf("[PAGBANK_WEBHOOK] assinatura inválida: %v", err)
		c.Status(http.StatusOK)
		return
	}

	payload, err := pagbank.ParseWebhookPayload(body)
	if err != nil {
		log.Printf("[PAGBANK_WEBHOOK] payload inválido: %v", err)
		c.Status(http.StatusOK)
		return
	}

	// reference_id é o nosso payment.ID (string numérica).
	referenceID := payload.Order.ReferenceID
	if referenceID == "" {
		c.Status(http.StatusOK)
		return
	}

	// Verifica se algum QR code ou charge está pago.
	isPaid := false
	var providerPaymentID string

	for _, qr := range payload.Order.QRCodes {
		if qr.Status == "PAID" {
			isPaid = true
			providerPaymentID = qr.ID
			break
		}
	}
	if !isPaid {
		for _, charge := range payload.Order.Charges {
			if charge.Status == "PAID" {
				isPaid = true
				providerPaymentID = charge.ID
				break
			}
		}
	}

	if !isPaid {
		c.Status(http.StatusOK)
		return
	}

	// Reutiliza o use case existente — que aceita externalReference (nosso payment.ID) e providerPaymentID.
	if err := h.markAsPaid.Execute(c.Request.Context(), referenceID, providerPaymentID); err != nil {
		log.Printf("[PAGBANK_WEBHOOK] markAsPaid error ref=%s provider_id=%s: %v", referenceID, providerPaymentID, err)
	}

	c.Status(http.StatusOK)
}

// getAnyPagBankToken retorna o access_token de qualquer barbearia com PagBank configurado.
// Usado apenas para buscar a public key do PagBank para validação de assinatura.
func (h *PagBankWebhookHandler) getAnyPagBankToken(_ *gin.Context) string {
	// Em sandbox, a assinatura não é enviada — retorna string vazia.
	if h.sandbox {
		return ""
	}
	// TODO: quando houver dados reais, ler um token válido para buscar a public key.
	// Por ora, retorna vazio — ValidateWebhookSignature só valida se signature != "".
	return ""
}

// referenceToPaymentID converte reference_id (string) para uint payment.ID.
func referenceToPaymentID(ref string) (uint, bool) {
	id, err := strconv.ParseUint(ref, 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(id), true
}
