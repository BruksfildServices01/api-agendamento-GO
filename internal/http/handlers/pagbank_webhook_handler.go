package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	paymentinfra "github.com/BruksfildServices01/barber-scheduler/internal/integration/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/integration/payment/pagbank"
	"github.com/BruksfildServices01/barber-scheduler/internal/security/crypt"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

// PagBankWebhookHandler processa notificações de pagamento do PagBank.
// Valida a assinatura RSA-SHA256 e delega a confirmação ao use case existente.
type PagBankWebhookHandler struct {
	db         *gorm.DB
	markAsPaid *ucPayment.MarkMPPaymentAsPaid
	cipher     *crypt.Cipher
	sandbox    bool
}

func NewPagBankWebhookHandler(
	db *gorm.DB,
	markAsPaid *ucPayment.MarkMPPaymentAsPaid,
	cipher *crypt.Cipher,
	sandbox bool,
) *PagBankWebhookHandler {
	return &PagBankWebhookHandler{
		db:         db,
		markAsPaid: markAsPaid,
		cipher:     cipher,
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

// getAnyPagBankToken retorna o access_token de qualquer barbearia com PagBank ativo.
// Usado apenas para buscar a public key RSA do PagBank para validar a assinatura do webhook.
// Qualquer token válido serve — a public key é a mesma para todo o ambiente.
func (h *PagBankWebhookHandler) getAnyPagBankToken(c *gin.Context) string {
	if h.sandbox || h.cipher == nil {
		return ""
	}

	var row struct {
		CredentialsEncrypted string `gorm:"column:credentials_encrypted"`
	}
	err := h.db.WithContext(c.Request.Context()).
		Table("barbershop_payment_providers").
		Select("credentials_encrypted").
		Where("provider = 'pagbank' AND enabled = true AND credentials_encrypted IS NOT NULL").
		Limit(1).
		Scan(&row).Error
	if err != nil || row.CredentialsEncrypted == "" {
		return ""
	}

	plaintext, err := h.cipher.Decrypt(row.CredentialsEncrypted)
	if err != nil {
		log.Printf("[PAGBANK_WEBHOOK] falha ao descriptografar token para validação RSA: %v", err)
		return ""
	}
	defer func() {
		for i := range plaintext {
			plaintext[i] = 0
		}
	}()

	var creds paymentinfra.PagBankCredentials
	if err := json.Unmarshal(plaintext, &creds); err != nil {
		return ""
	}
	return creds.AccessToken
}

// referenceToPaymentID converte reference_id (string) para uint payment.ID.
func referenceToPaymentID(ref string) (uint, bool) {
	id, err := strconv.ParseUint(ref, 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(id), true
}
