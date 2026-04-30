package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/crypt"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/mp"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// ErrPaymentNotConfigured é retornado quando a barbearia não tem um provider de pagamento configurado.
var ErrPaymentNotConfigured = errors.New("payment provider not configured for this barbershop")

// ProviderRegistry centraliza a seleção e instanciação do gateway de pagamento por barbearia.
// É o único lugar no sistema que conhece os providers disponíveis e suas credenciais.
//
// Ordem de resolução em TransparentGatewayFor:
//  1. barbershop_payment_providers — fonte principal, credenciais criptografadas com AES-256-GCM.
//  2. barbershop_payment_configs.mp_access_token — fallback legado, mantido como contingência.
//  3. Nenhum → ErrPaymentNotConfigured.
type ProviderRegistry struct {
	db     *gorm.DB
	cipher *crypt.Cipher // nil se PAYMENT_CREDENTIALS_ENCRYPTION_KEY não estiver configurada
}

func NewProviderRegistry(db *gorm.DB, cipher *crypt.Cipher) *ProviderRegistry {
	return &ProviderRegistry{db: db, cipher: cipher}
}

// TransparentGatewayFor retorna o gateway de checkout transparente para a barbearia.
//
// Se credentials_encrypted existir e a descriptografia falhar, retorna erro explícito —
// não cai silenciosamente no fallback, pois isso esconderia corrupção de credencial.
func (r *ProviderRegistry) TransparentGatewayFor(
	ctx context.Context,
	cfg models.BarbershopPaymentConfig,
) (domain.TransparentGateway, error) {

	// 1. Tenta provider na nova tabela.
	var p models.BarbershopPaymentProvider
	err := r.db.WithContext(ctx).
		Where("barbershop_id = ? AND provider = ? AND enabled = true", cfg.BarbershopID, "mercadopago").
		First(&p).Error

	if err == nil && p.CredentialsEncrypted != nil {
		gw, err := r.gatewayFromEncrypted(cfg.BarbershopID, *p.CredentialsEncrypted)
		if err != nil {
			// Falha explícita — não esconde corrupção com fallback silencioso.
			return nil, err
		}
		log.Printf("[PAYMENT] provider mercadopago carregado via provider table (barbershop=%d)", cfg.BarbershopID)
		return gw, nil
	}

	// 2. Fallback: usa mp_access_token da tabela antiga.
	if cfg.MPAccessToken != "" {
		log.Printf("[PAYMENT] provider mercadopago usando fallback legado (barbershop=%d)", cfg.BarbershopID)
		return mp.New(cfg.MPAccessToken)
	}

	return nil, ErrPaymentNotConfigured
}

// gatewayFromEncrypted descriptografa credentials_encrypted e retorna o gateway MP.
// Nunca loga o conteúdo das credenciais.
func (r *ProviderRegistry) gatewayFromEncrypted(barbershopID uint, encrypted string) (domain.TransparentGateway, error) {
	if r.cipher == nil {
		return nil, fmt.Errorf("registry: barbershop=%d tem credentials_encrypted mas PAYMENT_CREDENTIALS_ENCRYPTION_KEY não está configurada", barbershopID)
	}

	plaintext, err := r.cipher.Decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("registry: falha ao descriptografar credenciais (barbershop=%d)", barbershopID)
	}

	// Zera o plaintext da memória após uso.
	defer func() {
		for i := range plaintext {
			plaintext[i] = 0
		}
	}()

	var creds MPCredentials
	if err := json.Unmarshal(plaintext, &creds); err != nil {
		return nil, fmt.Errorf("registry: credenciais com formato inválido (barbershop=%d)", barbershopID)
	}

	if creds.AccessToken == "" {
		return nil, fmt.Errorf("registry: access_token vazio na nova tabela (barbershop=%d)", barbershopID)
	}

	return mp.New(creds.AccessToken)
}
