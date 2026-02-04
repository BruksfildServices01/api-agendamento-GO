package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//
// ======================================================
// PIX WEBHOOK AUTH MIDDLEWARE
//
// Responsabilidades:
// - Validar assinatura HMAC
// - Garantir integridade do payload
// - Bloquear chamadas não autorizadas
//
// NÃO faz:
// - parsing de payload
// - lógica de pagamento
// - side effects
// ======================================================
//

// Header padrão (ajuste se o gateway usar outro nome)
const PixSignatureHeader = "X-Pix-Signature"

// Algoritmo explícito (evita downgrade attack)
const PixSignatureAlgo = "sha256"

// NewPixWebhookAuth cria middleware de autenticação do webhook PIX.
//
// secret:
// - Deve vir de ENV (ex: PIX_WEBHOOK_SECRET)
// - Nunca hardcode
func NewPixWebhookAuth(secret string) gin.HandlerFunc {

	if secret == "" {
		// Fail fast em boot: webhook sem auth é falha crítica
		panic("PIX_WEBHOOK_SECRET is required")
	}

	secretBytes := []byte(secret)

	return func(c *gin.Context) {

		// --------------------------------------------------
		// 1️⃣ Extrair assinatura
		// --------------------------------------------------
		signature := c.GetHeader(PixSignatureHeader)
		if signature == "" {
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{"error": "missing_pix_signature"},
			)
			return
		}

		// --------------------------------------------------
		// 2️⃣ Ler body (precisa ser reusável)
		// --------------------------------------------------
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				gin.H{"error": "invalid_body"},
			)
			return
		}

		// Reinjeta o body para o handler seguinte
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		// --------------------------------------------------
		// 3️⃣ Calcular HMAC
		// --------------------------------------------------
		mac := hmac.New(sha256.New, secretBytes)
		mac.Write(body)
		expectedMAC := mac.Sum(nil)
		expectedSignature := hex.EncodeToString(expectedMAC)

		// --------------------------------------------------
		// 4️⃣ Comparação em tempo constante
		// --------------------------------------------------
		if !secureCompare(signature, expectedSignature) {
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{"error": "invalid_pix_signature"},
			)
			return
		}

		// --------------------------------------------------
		// 5️⃣ OK — segue pipeline
		// --------------------------------------------------
		c.Next()
	}
}

// secureCompare evita timing attacks
func secureCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}

	// normaliza (evita bypass por case)
	a = strings.ToLower(a)
	b = strings.ToLower(b)

	return hmac.Equal([]byte(a), []byte(b))
}
