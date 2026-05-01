package mp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// VerifyWebhookSignature valida o header x-signature enviado pelo Mercado Pago.
//
// Formato do header:
//
//	x-signature: ts=<timestamp>,v1=<hmac-sha256>
//
// Conteúdo assinado:
//
//	id=<dataID>&request-id=<xRequestID>&ts=<ts>
//
// Referência: https://www.mercadopago.com.br/developers/pt/docs/your-integrations/notifications/webhooks
func VerifyWebhookSignature(secret, xSignature, xRequestID, dataID string) bool {
	if secret == "" || xSignature == "" {
		return false
	}

	var ts, v1 string
	for _, part := range strings.Split(xSignature, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "ts=") {
			ts = strings.TrimPrefix(part, "ts=")
		} else if strings.HasPrefix(part, "v1=") {
			v1 = strings.TrimPrefix(part, "v1=")
		}
	}

	if ts == "" || v1 == "" {
		return false
	}

	manifest := fmt.Sprintf("id=%s&request-id=%s&ts=%s", dataID, xRequestID, ts)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(manifest))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(v1))
}
