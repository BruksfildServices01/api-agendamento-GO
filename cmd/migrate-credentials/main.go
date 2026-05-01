// cmd/migrate-credentials — migra credenciais Mercado Pago para barbershop_payment_providers.
//
// Uso: go run ./cmd/migrate-credentials
//
// Pré-requisitos:
//   - PAYMENT_CREDENTIALS_ENCRYPTION_KEY configurada (64 hex chars / 32 bytes AES-256).
//     Gerar com: openssl rand -hex 32
//   - DATABASE_URL apontando para o banco alvo.
//
// Comportamento:
//   - Busca barbearias com mp_access_token preenchido em barbershop_payment_configs.
//   - Para cada uma, criptografa as credenciais e faz UPSERT em barbershop_payment_providers.
//   - Operação idempotente: rodar duas vezes não cria duplicidade.
//   - Não remove mp_access_token nem mp_public_key.
//   - Nunca loga access_token, public_key ou o conteúdo de credentials_encrypted.
package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	dbpkg "github.com/BruksfildServices01/barber-scheduler/internal/db"
	"github.com/BruksfildServices01/barber-scheduler/internal/security/crypt"
	paymentinfra "github.com/BruksfildServices01/barber-scheduler/internal/infra/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	if cfg.PaymentCredentialsEncryptionKey == "" {
		log.Fatal("❌ PAYMENT_CREDENTIALS_ENCRYPTION_KEY não configurada. Gere com: openssl rand -hex 32")
	}

	cipher, err := crypt.NewCipher(cfg.PaymentCredentialsEncryptionKey)
	if err != nil {
		log.Fatalf("❌ Cipher inválido: %v", err)
	}

	db := dbpkg.NewDB(cfg)

	// Busca todas as barbearias com credenciais MP no fluxo antigo.
	var configs []models.BarbershopPaymentConfig
	if err := db.
		Where("mp_access_token IS NOT NULL AND mp_access_token <> ''").
		Find(&configs).Error; err != nil {
		log.Fatalf("❌ Erro ao consultar barbershop_payment_configs: %v", err)
	}

	log.Printf("[migrate-credentials] %d barbearia(s) com credenciais Mercado Pago para migrar", len(configs))

	migrated, failed := 0, 0

	for _, c := range configs {
		// Monta o payload de credenciais — nunca logar esse valor.
		raw, err := json.Marshal(paymentinfra.MPCredentials{
			AccessToken: c.MPAccessToken,
			PublicKey:   c.MPPublicKey,
		})
		if err != nil {
			log.Printf("[ERRO] barbershop_id=%d falha ao serializar credenciais: %v", c.BarbershopID, err)
			failed++
			continue
		}

		// Criptografa o payload.
		encrypted, err := cipher.Encrypt(raw)
		if err != nil {
			log.Printf("[ERRO] barbershop_id=%d falha ao criptografar: %v", c.BarbershopID, err)
			failed++
			continue
		}

		// Zera o plaintext da memória imediatamente após uso.
		for i := range raw {
			raw[i] = 0
		}

		// UPSERT idempotente — ON CONFLICT atualiza credentials_encrypted e enabled.
		if err := db.Exec(`
			INSERT INTO barbershop_payment_providers
				(barbershop_id, provider, enabled, environment, credentials_encrypted, created_at, updated_at)
			VALUES (?, 'mercadopago', true, 'production', ?, NOW(), NOW())
			ON CONFLICT (barbershop_id, provider) DO UPDATE SET
				credentials_encrypted = EXCLUDED.credentials_encrypted,
				enabled               = true,
				updated_at            = NOW()
		`, c.BarbershopID, encrypted).Error; err != nil {
			log.Printf("[ERRO] barbershop_id=%d falha no upsert: %v", c.BarbershopID, err)
			failed++
			continue
		}

		log.Printf("[OK] barbershop_id=%d migrado com sucesso", c.BarbershopID)
		migrated++
	}

	log.Printf("[migrate-credentials] Concluído — migrados=%d, falhas=%d", migrated, failed)

	if failed > 0 {
		os.Exit(1)
	}
}
