package payment

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/BruksfildServices01/barber-scheduler/internal/infra/crypt"
)

// testKey é uma chave de 64 hex chars (32 bytes AES-256) usada exclusivamente em testes.
const testRegistryKey = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

func newRegistryCipher(t *testing.T) *crypt.Cipher {
	t.Helper()
	c, err := crypt.NewCipher(testRegistryKey)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	return c
}

func encryptCreds(t *testing.T, c *crypt.Cipher, token, pubKey string) string {
	t.Helper()
	raw, err := json.Marshal(MPCredentials{AccessToken: token, PublicKey: pubKey})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	enc, err := c.Encrypt(raw)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	return enc
}

// TestRegistry_gatewayFromEncrypted_success valida que credenciais válidas
// criptografadas produzem um gateway sem erro.
func TestRegistry_gatewayFromEncrypted_success(t *testing.T) {
	c := newRegistryCipher(t)
	r := &ProviderRegistry{cipher: c}

	encrypted := encryptCreds(t, c, "TEST_ACCESS_TOKEN_FAKE", "TEST_PUBLIC_KEY_FAKE")

	gw, err := r.gatewayFromEncrypted(1, encrypted)
	if err != nil {
		t.Fatalf("gatewayFromEncrypted: %v", err)
	}
	if gw == nil {
		t.Fatal("gateway não deve ser nil em caso de sucesso")
	}
}

// TestRegistry_gatewayFromEncrypted_cipherNil valida que a ausência de cipher
// retorna erro claro sem cair em fallback silencioso.
func TestRegistry_gatewayFromEncrypted_cipherNil(t *testing.T) {
	r := &ProviderRegistry{cipher: nil}

	_, err := r.gatewayFromEncrypted(1, "qualquer-coisa")
	if err == nil {
		t.Fatal("deve retornar erro quando cipher é nil")
	}
	if !strings.Contains(err.Error(), "PAYMENT_CREDENTIALS_ENCRYPTION_KEY") {
		t.Errorf("erro deve mencionar PAYMENT_CREDENTIALS_ENCRYPTION_KEY, got: %v", err)
	}
}

// TestRegistry_gatewayFromEncrypted_wrongKey valida que descriptografar com chave
// errada retorna erro explícito — sem cair no fallback.
func TestRegistry_gatewayFromEncrypted_wrongKey(t *testing.T) {
	cEncrypt := newRegistryCipher(t)
	encrypted := encryptCreds(t, cEncrypt, "TOKEN", "PUBKEY")

	// Registry com chave diferente
	cWrong, err := crypt.NewCipher(strings.Repeat("cc", 32))
	if err != nil {
		t.Fatalf("NewCipher wrong key: %v", err)
	}
	r := &ProviderRegistry{cipher: cWrong}

	_, err = r.gatewayFromEncrypted(1, encrypted)
	if err == nil {
		t.Fatal("deve retornar erro ao descriptografar com chave errada")
	}
}

// TestRegistry_gatewayFromEncrypted_invalidJSON valida que ciphertext válido
// mas com JSON inválido dentro retorna erro de formato.
func TestRegistry_gatewayFromEncrypted_invalidJSON(t *testing.T) {
	c := newRegistryCipher(t)
	r := &ProviderRegistry{cipher: c}

	// Criptografa bytes que não são JSON válido
	encrypted, err := c.Encrypt([]byte("nao-e-json{{{"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = r.gatewayFromEncrypted(1, encrypted)
	if err == nil {
		t.Fatal("deve retornar erro quando JSON é inválido")
	}
	if !strings.Contains(err.Error(), "formato inválido") {
		t.Errorf("erro deve mencionar formato inválido, got: %v", err)
	}
}

// TestRegistry_gatewayFromEncrypted_emptyToken valida que access_token vazio
// dentro do payload retorna erro — não cria gateway inválido.
func TestRegistry_gatewayFromEncrypted_emptyToken(t *testing.T) {
	c := newRegistryCipher(t)
	r := &ProviderRegistry{cipher: c}

	// Credenciais com access_token vazio
	encrypted := encryptCreds(t, c, "", "some-pubkey")

	_, err := r.gatewayFromEncrypted(1, encrypted)
	if err == nil {
		t.Fatal("deve retornar erro quando access_token é vazio")
	}
	if !strings.Contains(err.Error(), "access_token vazio") {
		t.Errorf("erro deve mencionar access_token vazio, got: %v", err)
	}
}

// TestRegistry_ErrPaymentNotConfigured valida que o sentinel de erro está definido.
func TestRegistry_ErrPaymentNotConfigured(t *testing.T) {
	if ErrPaymentNotConfigured == nil {
		t.Fatal("ErrPaymentNotConfigured deve estar definido")
	}
	if ErrPaymentNotConfigured.Error() == "" {
		t.Fatal("ErrPaymentNotConfigured deve ter mensagem")
	}
}
