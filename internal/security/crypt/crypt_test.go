package crypt_test

import (
	"strings"
	"testing"

	"github.com/BruksfildServices01/barber-scheduler/internal/security/crypt"
)

// validKey é uma chave de teste de 64 hex chars (32 bytes).
// Nunca use esta chave fora de testes.
const validKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

func newTestCipher(t *testing.T) *crypt.Cipher {
	t.Helper()
	c, err := crypt.NewCipher(validKey)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	return c
}

// TestEncryptDecrypt_roundtrip valida que Encrypt → Decrypt recupera o plaintext original.
func TestEncryptDecrypt_roundtrip(t *testing.T) {
	c := newTestCipher(t)
	plaintext := []byte(`{"access_token":"tok_test","public_key":"pk_test"}`)

	encrypted, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	got, err := c.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if string(got) != string(plaintext) {
		t.Fatalf("roundtrip mismatch: got %q, want %q", got, plaintext)
	}
}

// TestEncrypt_uniquePerCall valida que dois Encrypt do mesmo plaintext
// produzem ciphertexts distintos (nonce aleatório por chamada).
func TestEncrypt_uniquePerCall(t *testing.T) {
	c := newTestCipher(t)
	plaintext := []byte("mesmo plaintext")

	e1, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 1: %v", err)
	}
	e2, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 2: %v", err)
	}

	if e1 == e2 {
		t.Fatal("dois Encrypt do mesmo plaintext não podem gerar o mesmo ciphertext (nonce deve ser aleatório)")
	}
}

// TestDecrypt_wrongKey valida que descriptografar com chave diferente retorna erro.
func TestDecrypt_wrongKey(t *testing.T) {
	c1 := newTestCipher(t)

	encrypted, err := c1.Encrypt([]byte("credencial secreta"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	c2, err := crypt.NewCipher(strings.Repeat("ff", 32))
	if err != nil {
		t.Fatalf("NewCipher chave errada: %v", err)
	}

	_, err = c2.Decrypt(encrypted)
	if err == nil {
		t.Fatal("Decrypt com chave errada deve retornar erro")
	}
}

// TestDecrypt_invalidBase64 valida que payload não-base64 retorna erro.
func TestDecrypt_invalidBase64(t *testing.T) {
	c := newTestCipher(t)
	_, err := c.Decrypt("isso-nao-e-base64-valido!!!")
	if err == nil {
		t.Fatal("Decrypt com base64 inválido deve retornar erro")
	}
}

// TestDecrypt_tooShort valida que payload base64 válido mas curto demais retorna erro.
func TestDecrypt_tooShort(t *testing.T) {
	c := newTestCipher(t)
	// "short" em base64 — válido mas menor que o nonce mínimo (12 bytes)
	_, err := c.Decrypt("c2hvcnQ=")
	if err == nil {
		t.Fatal("Decrypt com payload curto demais deve retornar erro")
	}
}

// TestDecrypt_emptyString valida que string vazia retorna erro.
func TestDecrypt_emptyString(t *testing.T) {
	c := newTestCipher(t)
	_, err := c.Decrypt("")
	if err == nil {
		t.Fatal("Decrypt com string vazia deve retornar erro")
	}
}

// TestDecrypt_tamperedPayload valida que ciphertext adulterado retorna erro (tag GCM falha).
func TestDecrypt_tamperedPayload(t *testing.T) {
	c := newTestCipher(t)

	encrypted, err := c.Encrypt([]byte("dado original"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Adultera o último caractere do base64
	tampered := encrypted[:len(encrypted)-1] + "X"
	_, err = c.Decrypt(tampered)
	if err == nil {
		t.Fatal("Decrypt de payload adulterado deve retornar erro")
	}
}

// TestNewCipher_invalidHex valida que chave com caracteres não-hex retorna erro.
func TestNewCipher_invalidHex(t *testing.T) {
	_, err := crypt.NewCipher("isso-nao-e-hex-valido")
	if err == nil {
		t.Fatal("NewCipher com hex inválido deve retornar erro")
	}
}

// TestNewCipher_wrongLength valida que chave com tamanho errado retorna erro.
// AES-128 (16 bytes / 32 hex chars) não é aceito — exigimos AES-256 (32 bytes).
func TestNewCipher_wrongLength(t *testing.T) {
	_, err := crypt.NewCipher(strings.Repeat("01", 16)) // 16 bytes = AES-128
	if err == nil {
		t.Fatal("NewCipher com chave de 16 bytes deve retornar erro — exige 32 bytes (AES-256)")
	}
}
