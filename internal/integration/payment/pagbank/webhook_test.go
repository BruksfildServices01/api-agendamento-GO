package pagbank

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

// ── Setup ─────────────────────────────────────────────────────────────────────

// testKey é gerado uma vez para todos os testes deste pacote.
// Geração de chave RSA 2048-bit leva ~100ms — centralizar evita repetição.
var testKey *rsa.PrivateKey

func TestMain(m *testing.M) {
	var err error
	testKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic("falha ao gerar chave RSA de teste: " + err.Error())
	}
	os.Exit(m.Run())
}

// injectPublicKey injeta uma chave pública diretamente no cache do pacote,
// simulando uma busca já realizada. Retorna uma função de cleanup que restaura
// o estado original do cache.
func injectPublicKey(t *testing.T, pub *rsa.PublicKey) {
	t.Helper()
	pbKeyCache.mu.Lock()
	prev := pbKeyCache.key
	prevAt := pbKeyCache.fetchedAt
	pbKeyCache.key = pub
	pbKeyCache.fetchedAt = time.Now()
	pbKeyCache.mu.Unlock()

	t.Cleanup(func() {
		pbKeyCache.mu.Lock()
		pbKeyCache.key = prev
		pbKeyCache.fetchedAt = prevAt
		pbKeyCache.mu.Unlock()
	})
}

// clearPublicKeyCache força o cache a expirar, garantindo que fetchPublicKey
// seja chamado. Retorna cleanup que restaura o estado.
func clearPublicKeyCache(t *testing.T) {
	t.Helper()
	pbKeyCache.mu.Lock()
	prev := pbKeyCache.key
	prevAt := pbKeyCache.fetchedAt
	pbKeyCache.key = nil
	pbKeyCache.fetchedAt = time.Time{}
	pbKeyCache.mu.Unlock()

	t.Cleanup(func() {
		pbKeyCache.mu.Lock()
		pbKeyCache.key = prev
		pbKeyCache.fetchedAt = prevAt
		pbKeyCache.mu.Unlock()
	})
}

// replaceHTTPClient substitui o httpClient do pacote pelo fornecido e restaura
// o original via t.Cleanup.
func replaceHTTPClient(t *testing.T, client *http.Client) {
	t.Helper()
	orig := httpClient
	httpClient = client
	t.Cleanup(func() { httpClient = orig })
}

// signPayload assina payload com a chave privada usando RSA-PKCS1v15-SHA256
// e retorna a assinatura em base64, exatamente como o PagBank faria.
func signPayload(t *testing.T, key *rsa.PrivateKey, payload []byte) string {
	t.Helper()
	hash := sha256.Sum256(payload)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		t.Fatalf("falha ao assinar payload: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

// publicKeyToPEM serializa uma chave pública RSA em formato PEM PKIX.
func publicKeyToPEM(pub *rsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: der}
	return string(pem.EncodeToMemory(block)), nil
}

// newPublicKeyServer inicia um httptest.Server que serve a chave pública da
// testKey no formato esperado pela PagBank (GET /public-keys/notifications).
// O servidor usa um sync.Once para serializar o estado do cache em testes
// que precisam forçar o fetch HTTP.
func newPublicKeyServer(t *testing.T, key *rsa.PublicKey) *httptest.Server {
	t.Helper()
	pemStr, err := publicKeyToPEM(key)
	if err != nil {
		t.Fatalf("falha ao serializar chave pública: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/public-keys/notifications" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pagbankPublicKeyResponse{PublicKey: pemStr})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ── Casos positivos ────────────────────────────────────────────────────────────

func TestValidateWebhookSignature_EmptySignatureSandboxReturnsNil(t *testing.T) {
	// Sandbox não envia assinatura — comportamento documentado: nil é retornado.
	err := ValidateWebhookSignature(context.Background(), "any-token", "", []byte("payload"), true)
	if err != nil {
		t.Errorf("assinatura vazia em sandbox deve retornar nil, obteve: %v", err)
	}
}

func TestValidateWebhookSignature_EmptySignatureOutsideSandboxReturnsError(t *testing.T) {
	// Fora do sandbox, assinatura ausente deve ser rejeitada — webhook forjado.
	err := ValidateWebhookSignature(context.Background(), "any-token", "", []byte("payload"), false)
	if err == nil {
		t.Error("assinatura vazia fora do sandbox deve retornar erro")
	}
}

func TestValidateWebhookSignature_ValidRSASignature(t *testing.T) {
	payload := []byte(`{"order":{"id":"ORD_123","reference_id":"456"}}`)
	sig := signPayload(t, testKey, payload)
	injectPublicKey(t, &testKey.PublicKey)

	err := ValidateWebhookSignature(context.Background(), "token", sig, payload, false)
	if err != nil {
		t.Errorf("assinatura RSA válida deve ser aceita, obteve: %v", err)
	}
}

func TestValidateWebhookSignature_FetchesPublicKeyViaHTTP(t *testing.T) {
	// Testa o caminho completo: cache vazio → fetch HTTP → validação RSA.
	clearPublicKeyCache(t)

	srv := newPublicKeyServer(t, &testKey.PublicKey)
	// Substitui baseURLProd/Sandbox apontando para o servidor de teste.
	// httpClient aponta para o test server via URL absoluta; usamos um Transport
	// que redireciona qualquer host para o servidor de teste.
	replaceHTTPClient(t, &http.Client{
		Transport: rewriteHostTransport(srv.URL),
		Timeout:   5 * time.Second,
	})

	payload := []byte(`{"order":{"id":"ORD_FETCH","reference_id":"789"}}`)
	sig := signPayload(t, testKey, payload)

	err := ValidateWebhookSignature(context.Background(), "test-token", sig, payload, false)
	if err != nil {
		t.Errorf("validação com fetch HTTP deve funcionar, obteve: %v", err)
	}
}

// ── Casos negativos — assinatura inválida ─────────────────────────────────────

func TestValidateWebhookSignature_InvalidBase64Signature(t *testing.T) {
	injectPublicKey(t, &testKey.PublicKey)

	err := ValidateWebhookSignature(context.Background(), "token", "not-valid-base64!!!", []byte("payload"), false)
	if err == nil {
		t.Error("assinatura base64 inválida deve retornar erro")
	}
}

func TestValidateWebhookSignature_WrongRSASignature(t *testing.T) {
	// Assinatura em base64 válida, mas bytes errados.
	injectPublicKey(t, &testKey.PublicKey)
	wrongSig := base64.StdEncoding.EncodeToString(make([]byte, 256))

	err := ValidateWebhookSignature(context.Background(), "token", wrongSig, []byte("payload"), false)
	if err == nil {
		t.Error("assinatura RSA inválida deve retornar erro")
	}
}

func TestValidateWebhookSignature_TamperedPayload(t *testing.T) {
	// Assina o payload original, mas envia payload adulterado na verificação.
	original := []byte(`{"order":{"id":"ORD_ORIG"}}`)
	tampered := []byte(`{"order":{"id":"ORD_TAMPERED"}}`)

	sig := signPayload(t, testKey, original)
	injectPublicKey(t, &testKey.PublicKey)

	err := ValidateWebhookSignature(context.Background(), "token", sig, tampered, false)
	if err == nil {
		t.Error("payload adulterado deve invalidar a assinatura RSA")
	}
}

func TestValidateWebhookSignature_SignedWithDifferentKey(t *testing.T) {
	// Webhook assinado com chave diferente da que está no cache.
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("falha ao gerar chave alternativa: %v", err)
	}

	payload := []byte(`{"order":{"id":"ORD_OTHER"}}`)
	sig := signPayload(t, otherKey, payload) // assina com outra chave
	injectPublicKey(t, &testKey.PublicKey)   // cache tem a chave original

	err = ValidateWebhookSignature(context.Background(), "token", sig, payload, false)
	if err == nil {
		t.Error("assinatura de chave diferente deve ser rejeitada")
	}
}

func TestValidateWebhookSignature_FetchPublicKeyFailure(t *testing.T) {
	clearPublicKeyCache(t)

	// Servidor que retorna 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	replaceHTTPClient(t, &http.Client{
		Transport: rewriteHostTransport(srv.URL),
		Timeout:   5 * time.Second,
	})

	err := ValidateWebhookSignature(context.Background(), "token", "anybase64==", []byte("payload"), false)
	if err == nil {
		t.Error("falha ao obter chave pública deve retornar erro")
	}
}

func TestValidateWebhookSignature_FetchPublicKeyServerUnreachable(t *testing.T) {
	clearPublicKeyCache(t)

	// Aponta para porta não utilizada — conexão recusada.
	replaceHTTPClient(t, &http.Client{
		Transport: rewriteHostTransport("http://127.0.0.1:1"),
		Timeout:   2 * time.Second,
	})

	err := ValidateWebhookSignature(context.Background(), "token", "anybase64==", []byte("payload"), false)
	if err == nil {
		t.Error("servidor inacessível deve retornar erro")
	}
}

// ── Casos de segurança — sem panic ────────────────────────────────────────────

func TestValidateWebhookSignature_MalformedSignatureNoPanic(t *testing.T) {
	injectPublicKey(t, &testKey.PublicKey)

	cases := []string{
		"garbage",
		"   ",
		"\x00\x01\x02\x03",
		"====",                      // base64 com padding mas sem dados
		string(make([]byte, 10000)), // string gigante
		"AAAA",                      // base64 válido mas bytes insuficientes para RSA
	}

	for _, sig := range cases {
		// Nenhum caso pode gerar panic.
		_ = ValidateWebhookSignature(context.Background(), "token", sig, []byte("payload"), false)
	}
}

// ── ParseWebhookPayload ───────────────────────────────────────────────────────

func TestParseWebhookPayload_ValidPayload(t *testing.T) {
	body := []byte(`{
		"order": {
			"id": "ORD_ABC123",
			"reference_id": "789",
			"qr_codes": [{"id": "QRC_1", "status": "PAID"}],
			"charges": []
		}
	}`)

	p, err := ParseWebhookPayload(body)
	if err != nil {
		t.Fatalf("payload válido não deve retornar erro: %v", err)
	}
	if p.Order.ID != "ORD_ABC123" {
		t.Errorf("esperado order.id=ORD_ABC123, obtido=%q", p.Order.ID)
	}
	if p.Order.ReferenceID != "789" {
		t.Errorf("esperado reference_id=789, obtido=%q", p.Order.ReferenceID)
	}
	if len(p.Order.QRCodes) != 1 || p.Order.QRCodes[0].Status != "PAID" {
		t.Errorf("qr_code.status esperado=PAID, obtido=%+v", p.Order.QRCodes)
	}
}

func TestParseWebhookPayload_InvalidJSON(t *testing.T) {
	_, err := ParseWebhookPayload([]byte("not-json{"))
	if err == nil {
		t.Error("JSON inválido deve retornar erro")
	}
}

func TestParseWebhookPayload_EmptyBody(t *testing.T) {
	// JSON vazio é válido estruturalmente mas resulta em struct zerada.
	p, err := ParseWebhookPayload([]byte("{}"))
	if err != nil {
		t.Errorf("body {} não deve retornar erro: %v", err)
	}
	if p.Order.ID != "" {
		t.Errorf("esperado order.id vazio, obtido=%q", p.Order.ID)
	}
}

// ── Helpers internos ──────────────────────────────────────────────────────────

// rewriteHostTransport retorna um RoundTripper que redireciona todas as
// requisições para targetBase, preservando path e query string.
// Permite usar httptest.Server sem alterar as constantes de URL de produção.
func rewriteHostTransport(targetBase string) http.RoundTripper {
	return &hostRewriteTransport{base: targetBase}
}

type hostRewriteTransport struct {
	base string
	once sync.Once
	tr   *http.Transport
}

func (t *hostRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.once.Do(func() { t.tr = &http.Transport{} })
	rewritten := req.Clone(req.Context())
	rewritten.URL.Scheme = "http"
	rewritten.URL.Host = t.base[len("http://"):]
	rewritten.Host = rewritten.URL.Host
	return t.tr.RoundTrip(rewritten)
}

// ── WebhookPayload type check ─────────────────────────────────────────────────

func TestWebhookPayload_TypeStructure(t *testing.T) {
	// Garante que WebhookPayload tem os campos esperados pelo handler.
	// Se a struct for alterada, este teste ajuda a detectar o impacto.
	body := []byte(fmt.Sprintf(`{
		"order": {
			"id": "ORD_1",
			"reference_id": "42",
			"qr_codes": [{"id":"QRC_1","status":"PAID"}],
			"charges": [{"id":"CHAR_1","status":"DECLINED"}]
		}
	}`))

	p, err := ParseWebhookPayload(body)
	if err != nil {
		t.Fatalf("parse falhou: %v", err)
	}
	if p.Order.QRCodes[0].ID != "QRC_1" {
		t.Errorf("qr_codes[0].id esperado=QRC_1, obtido=%q", p.Order.QRCodes[0].ID)
	}
	if p.Order.Charges[0].Status != "DECLINED" {
		t.Errorf("charges[0].status esperado=DECLINED, obtido=%q", p.Order.Charges[0].Status)
	}
}
