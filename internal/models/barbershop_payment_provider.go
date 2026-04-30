package models

import "time"

// BarbershopPaymentProvider representa um provider de pagamento configurado
// por barbearia na tabela barbershop_payment_providers.
//
// credentials_encrypted: AES-GCM criptografado na aplicação, armazenado em base64.
// Será NULL enquanto os dados não forem migrados da tabela antiga.
// A criptografia/descriptografia será implementada na Fase 4B.
//
// webhook_secret: separado para leitura em hot path (validação HMAC de webhook)
// sem necessidade de descriptografar credentials_encrypted.
// Nunca deve ser serializado em JSON ou logado.
type BarbershopPaymentProvider struct {
	ID           uint   `gorm:"primaryKey"`
	BarbershopID uint   `gorm:"not null;index"`
	Provider     string `gorm:"size:50;not null"`
	Enabled      bool   `gorm:"not null;default:false"`
	Environment  string `gorm:"size:20;not null;default:production"`

	// CredentialsEncrypted é NULL até que os dados sejam migrados (Fase 4B).
	CredentialsEncrypted *string `gorm:"type:text"`

	// WebhookSecret nunca deve aparecer em DTOs ou logs.
	WebhookSecret *string `gorm:"size:500" json:"-"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
