// Package crypt fornece criptografia simétrica AES-256-GCM para armazenamento
// seguro de credenciais de providers de pagamento em banco de dados.
//
// Formato do ciphertext armazenado:
//
//	base64.StdEncoding( nonce[12] || ciphertext[n] || tag[16] )
//
// O nonce é gerado aleatoriamente por chamada, garantindo que dois Encrypt
// do mesmo plaintext produzam ciphertexts diferentes.
// O tag GCM detecta qualquer adulteração — Decrypt falha com erro explícito.
package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// Cipher encapsula uma chave AES-256 e expõe operações de encrypt/decrypt.
// Deve ser instanciado uma vez na inicialização e injetado onde necessário.
// Nunca logar ou serializar a chave interna.
type Cipher struct {
	key []byte // 32 bytes, AES-256
}

// NewCipher cria um Cipher a partir de uma chave hexadecimal de 64 caracteres (32 bytes).
// Retorna erro se hexKey for inválido ou não tiver exatamente 32 bytes.
//
// Para gerar uma chave segura: openssl rand -hex 32
func NewCipher(hexKey string) (*Cipher, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("crypt: chave hexadecimal inválida: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("crypt: chave deve ter 32 bytes (64 hex chars para AES-256), recebidos %d bytes", len(key))
	}
	return &Cipher{key: key}, nil
}

// Encrypt criptografa plaintext com AES-256-GCM e retorna o resultado em base64.
// Cada chamada gera um nonce aleatório, produzindo ciphertexts distintos mesmo
// para o mesmo plaintext de entrada.
func (c *Cipher) Encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("crypt: aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypt: aes gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypt: geração de nonce: %w", err)
	}

	// Seal(dst, nonce, plaintext, aad) → appends nonce + ciphertext + tag
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt descriptografa um ciphertext base64 gerado por Encrypt.
// Retorna erro se o payload for inválido, corrompido ou descriptografado com chave errada.
// O erro é intencionalmente genérico para não vazar informações sobre a falha.
func (c *Cipher) Decrypt(encoded string) ([]byte, error) {
	if encoded == "" {
		return nil, errors.New("crypt: payload vazio")
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("crypt: payload base64 inválido")
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("crypt: aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypt: aes gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("crypt: payload muito curto")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// Erro genérico: não revelar se falhou por chave errada, payload corrompido etc.
		return nil, errors.New("crypt: falha na descriptografia — payload inválido ou chave incorreta")
	}
	return plaintext, nil
}
