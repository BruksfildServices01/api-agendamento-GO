package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/security/crypt"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/mp"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/pagbank"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// ErrPaymentNotConfigured é retornado quando a barbearia não tem um provider de pagamento configurado.
var ErrPaymentNotConfigured = errors.New("payment provider not configured for this barbershop")

// ProviderRegistry centraliza a seleção e instanciação do gateway de pagamento por barbearia.
// É o único lugar no sistema que conhece os providers disponíveis e suas credenciais.
//
// Ordem de resolução em TransparentGatewayFor:
//  1. barbershop_payment_providers — fonte principal, credenciais criptografadas com AES-256-GCM.
//     Suporta: mercadopago, pagbank.
//  2. barbershop_payment_configs.mp_access_token — fallback legado Mercado Pago.
//  3. Nenhum → ErrPaymentNotConfigured.
type ProviderRegistry struct {
	db             *gorm.DB
	cipher         *crypt.Cipher // nil se PAYMENT_CREDENTIALS_ENCRYPTION_KEY não configurada
	pagbankSandbox bool
}

func NewProviderRegistry(db *gorm.DB, cipher *crypt.Cipher, pagbankSandbox bool) *ProviderRegistry {
	return &ProviderRegistry{db: db, cipher: cipher, pagbankSandbox: pagbankSandbox}
}

// TransparentGatewayFor retorna o gateway de checkout transparente para a barbearia.
//
// Se credentials_encrypted existir e a descriptografia falhar, retorna erro explícito —
// não cai silenciosamente no fallback, pois isso esconderia corrupção de credencial.
func (r *ProviderRegistry) TransparentGatewayFor(
	ctx context.Context,
	cfg models.BarbershopPaymentConfig,
) (domain.TransparentGateway, error) {

	// 1. Tenta qualquer provider habilitado na nova tabela.
	var p models.BarbershopPaymentProvider
	err := r.db.WithContext(ctx).
		Where("barbershop_id = ? AND enabled = true", cfg.BarbershopID).
		Order("updated_at DESC"). // provider mais recentemente configurado tem prioridade
		First(&p).Error

	if err == nil && p.CredentialsEncrypted != nil {
		gw, err := r.gatewayFromProvider(cfg.BarbershopID, p)
		if err != nil {
			return nil, err
		}
		log.Printf("[PAYMENT] provider %s carregado via provider table (barbershop=%d)", p.Provider, cfg.BarbershopID)
		return gw, nil
	}

	// 2. Fallback legado: usa mp_access_token da tabela antiga (Mercado Pago).
	if cfg.MPAccessToken != "" {
		log.Printf("[PAYMENT] provider mercadopago usando fallback legado (barbershop=%d)", cfg.BarbershopID)
		return mp.New(cfg.MPAccessToken)
	}

	return nil, ErrPaymentNotConfigured
}

// gatewayFromProvider determina o provider e instancia o gateway correto.
func (r *ProviderRegistry) gatewayFromProvider(barbershopID uint, p models.BarbershopPaymentProvider) (domain.TransparentGateway, error) {
	switch p.Provider {
	case "mercadopago":
		return r.gatewayFromEncrypted(barbershopID, *p.CredentialsEncrypted)
	case "pagbank":
		return r.pagbankGatewayFromEncrypted(barbershopID, *p.CredentialsEncrypted)
	default:
		return nil, fmt.Errorf("registry: provider desconhecido %q (barbershop=%d)", p.Provider, barbershopID)
	}
}

// gatewayFromEncrypted descriptografa credentials_encrypted e retorna o gateway MP.
func (r *ProviderRegistry) gatewayFromEncrypted(barbershopID uint, encrypted string) (domain.TransparentGateway, error) {
	plaintext, err := r.decrypt(barbershopID, encrypted)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(plaintext)

	var creds MPCredentials
	if err := json.Unmarshal(plaintext, &creds); err != nil {
		return nil, fmt.Errorf("registry: credenciais MP com formato inválido (barbershop=%d)", barbershopID)
	}
	if creds.AccessToken == "" {
		return nil, fmt.Errorf("registry: access_token vazio (barbershop=%d)", barbershopID)
	}
	return mp.New(creds.AccessToken)
}

// pagbankGatewayFromEncrypted descriptografa credentials_encrypted e retorna o gateway PagBank.
func (r *ProviderRegistry) pagbankGatewayFromEncrypted(barbershopID uint, encrypted string) (domain.TransparentGateway, error) {
	plaintext, err := r.decrypt(barbershopID, encrypted)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(plaintext)

	var creds PagBankCredentials
	if err := json.Unmarshal(plaintext, &creds); err != nil {
		return nil, fmt.Errorf("registry: credenciais PagBank com formato inválido (barbershop=%d)", barbershopID)
	}
	if creds.AccessToken == "" {
		return nil, fmt.Errorf("registry: access_token PagBank vazio (barbershop=%d)", barbershopID)
	}
	return pagbank.New(creds.AccessToken, r.pagbankSandbox)
}

// decrypt centraliza a descriptografia, validando o cipher e sem logar segredos.
func (r *ProviderRegistry) decrypt(barbershopID uint, encrypted string) ([]byte, error) {
	if r.cipher == nil {
		return nil, fmt.Errorf("registry: barbershop=%d tem credentials_encrypted mas PAYMENT_CREDENTIALS_ENCRYPTION_KEY não está configurada", barbershopID)
	}
	plaintext, err := r.cipher.Decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("registry: falha ao descriptografar credenciais (barbershop=%d)", barbershopID)
	}
	return plaintext, nil
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
