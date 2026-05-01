package payment_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/BruksfildServices01/barber-scheduler/internal/security/crypt"
	paymentinfra "github.com/BruksfildServices01/barber-scheduler/internal/integration/payment"
)

const testKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

func newTestCipher(t *testing.T) *crypt.Cipher {
	t.Helper()
	c, err := crypt.NewCipher(testKey)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	return c
}

// TestMPCredentials_encryptDecryptRoundtrip valida que o payload de credenciais
// sobrevive ao ciclo marshal → encrypt → decrypt → unmarshal com os campos intactos.
func TestMPCredentials_encryptDecryptRoundtrip(t *testing.T) {
	cipher := newTestCipher(t)

	original := paymentinfra.MPCredentials{
		AccessToken: "APP_USR-token-test-123",
		PublicKey:   "APP_USR-pubkey-test-456",
	}

	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	encrypted, err := cipher.Encrypt(raw)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Simula o que o registry faz ao ler da nova tabela.
	plaintext, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	var recovered paymentinfra.MPCredentials
	if err := json.Unmarshal(plaintext, &recovered); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if recovered.AccessToken != original.AccessToken {
		t.Errorf("AccessToken mismatch: got %q, want %q", recovered.AccessToken, original.AccessToken)
	}
	if recovered.PublicKey != original.PublicKey {
		t.Errorf("PublicKey mismatch: got %q, want %q", recovered.PublicKey, original.PublicKey)
	}
}

// TestMPCredentials_encryptedDoesNotContainPlaintext garante que o valor
// criptografado não contém access_token nem public_key em texto puro.
func TestMPCredentials_encryptedDoesNotContainPlaintext(t *testing.T) {
	cipher := newTestCipher(t)

	creds := paymentinfra.MPCredentials{
		AccessToken: "SECRET_ACCESS_TOKEN_SHOULD_NOT_APPEAR",
		PublicKey:   "SECRET_PUBLIC_KEY_SHOULD_NOT_APPEAR",
	}

	raw, _ := json.Marshal(creds)
	encrypted, err := cipher.Encrypt(raw)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if strings.Contains(encrypted, creds.AccessToken) {
		t.Error("credentials_encrypted não deve conter access_token em texto puro")
	}
	if strings.Contains(encrypted, creds.PublicKey) {
		t.Error("credentials_encrypted não deve conter public_key em texto puro")
	}
}

// TestMPCredentials_wrongKeyCannotDecrypt garante que o payload criptografado
// com uma chave não pode ser descriptografado com outra.
func TestMPCredentials_wrongKeyCannotDecrypt(t *testing.T) {
	cipher1 := newTestCipher(t)

	raw, _ := json.Marshal(paymentinfra.MPCredentials{AccessToken: "tok", PublicKey: "pk"})
	encrypted, err := cipher1.Encrypt(raw)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	cipher2, err := crypt.NewCipher(strings.Repeat("ab", 32))
	if err != nil {
		t.Fatalf("NewCipher wrong key: %v", err)
	}

	_, err = cipher2.Decrypt(encrypted)
	if err == nil {
		t.Fatal("Decrypt com chave diferente deve retornar erro")
	}
}
