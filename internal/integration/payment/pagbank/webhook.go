package pagbank

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// publicKeyCache armazena a chave pública da PagBank para validação de webhooks.
// A chave é buscada uma vez e cacheada por 24h.
type publicKeyCache struct {
	mu        sync.RWMutex
	key       *rsa.PublicKey
	fetchedAt time.Time
	ttl       time.Duration
}

var pbKeyCache = &publicKeyCache{ttl: 24 * time.Hour}

type pagbankPublicKeyResponse struct {
	PublicKey string `json:"public_key"`
}

// ValidateWebhookSignature valida o header x-payload-signature de um webhook PagBank.
// Usa RSA-SHA256 com a chave pública da PagBank (buscada e cacheada).
// Retorna nil se a assinatura for válida.
//
// Nota: em sandbox, a PagBank não envia o header — passar assinatura vazia retorna nil.
func ValidateWebhookSignature(ctx context.Context, accessToken, signature string, payload []byte, sandbox bool) error {
	if signature == "" {
		// Sandbox não envia assinatura — aceita sem validar.
		return nil
	}

	key, err := pbKeyCache.get(ctx, accessToken, sandbox)
	if err != nil {
		return fmt.Errorf("pagbank webhook: falha ao obter chave pública: %w", err)
	}

	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("pagbank webhook: assinatura base64 inválida")
	}

	hash := sha256.Sum256(payload)
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, hash[:], sig); err != nil {
		return fmt.Errorf("pagbank webhook: assinatura inválida")
	}
	return nil
}

// ParseWebhookPayload desserializa o corpo do webhook PagBank.
func ParseWebhookPayload(body []byte) (*WebhookPayload, error) {
	var p WebhookPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("pagbank webhook: payload inválido: %w", err)
	}
	return &p, nil
}

// ── cache ──────────────────────────────────────────────────────────────────────

func (c *publicKeyCache) get(ctx context.Context, accessToken string, sandbox bool) (*rsa.PublicKey, error) {
	c.mu.RLock()
	if c.key != nil && time.Since(c.fetchedAt) < c.ttl {
		k := c.key
		c.mu.RUnlock()
		return k, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check após adquirir write lock.
	if c.key != nil && time.Since(c.fetchedAt) < c.ttl {
		return c.key, nil
	}

	key, err := fetchPublicKey(ctx, accessToken, sandbox)
	if err != nil {
		return nil, err
	}
	c.key = key
	c.fetchedAt = time.Now()
	return key, nil
}

func fetchPublicKey(ctx context.Context, accessToken string, sandbox bool) (*rsa.PublicKey, error) {
	base := baseURLProd
	if sandbox {
		base = baseURLSandbox
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/public-keys/notifications", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch public key: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch public key %d: %s", resp.StatusCode, string(data))
	}

	var pkResp pagbankPublicKeyResponse
	if err := json.Unmarshal(data, &pkResp); err != nil {
		return nil, fmt.Errorf("parse public key response: %w", err)
	}

	return parseRSAPublicKey(pkResp.PublicKey)
}

func parseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("pagbank: chave pública PEM inválida")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("pagbank: parse public key: %w", err)
	}

	rsaKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("pagbank: chave pública não é RSA")
	}
	return rsaKey, nil
}
