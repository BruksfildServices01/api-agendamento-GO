package calendar

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/BruksfildServices01/barber-scheduler/internal/security/crypt"
)

// SyncAppointmentToGoogle cria um evento no Google Calendar do barbeiro
// de forma assíncrona (best-effort — falhas são apenas logadas).
// cipher é usado para descriptografar os tokens armazenados no banco;
// nil desativa a criptografia (modo dev sem PAYMENT_CREDENTIALS_ENCRYPTION_KEY).
func SyncAppointmentToGoogle(
	db *gorm.DB,
	cfg OAuthConfig,
	cipher *crypt.Cipher,
	barberID uint,
	barbershopID uint,
	ap *models.Appointment,
) {
	if cfg.ClientID == "" {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := syncAppointment(ctx, db, cfg, cipher, barberID, barbershopID, ap); err != nil {
			log.Printf("[GOOGLE_CAL] sync failed barber=%d appointment=%d: %v",
				barberID, ap.ID, err)
		}
	}()
}

func syncAppointment(
	ctx context.Context,
	db *gorm.DB,
	cfg OAuthConfig,
	cipher *crypt.Cipher,
	barberID uint,
	barbershopID uint,
	ap *models.Appointment,
) error {
	// Carrega token válido do barbeiro
	var token models.BarberGoogleToken
	if err := db.WithContext(ctx).Where("user_id = ?", barberID).First(&token).Error; err != nil {
		return nil // barbeiro não conectou Google — sem erro
	}

	accessToken, err := ensureValidToken(ctx, db, cfg, cipher, &token)
	if err != nil {
		return fmt.Errorf("token: %w", err)
	}

	// Carrega dados adicionais para montar o evento
	summary, description := buildEventText(ctx, db, ap)

	// Carrega timezone da barbearia
	timezone := loadTimezone(ctx, db, barbershopID)

	return CreateEvent(ctx, accessToken, EventInput{
		Summary:     summary,
		Description: description,
		Start:       ap.StartTime,
		End:         ap.EndTime,
		Timezone:    timezone,
	})
}

func ensureValidToken(ctx context.Context, db *gorm.DB, cfg OAuthConfig, cipher *crypt.Cipher, token *models.BarberGoogleToken) (string, error) {
	// Descriptografa os tokens armazenados no banco.
	// Se a descriptografia falhar (token antigo em texto puro), usa o valor como está
	// e o re-salva criptografado para migração transparente.
	accessPlain, accessWasPlain := DecryptField(cipher, token.AccessToken)
	refreshPlain, refreshWasPlain := DecryptField(cipher, token.RefreshToken)

	if accessWasPlain || refreshWasPlain {
		// Token antigo em texto puro — re-salva criptografado imediatamente
		if encAcc, err := EncryptField(cipher, accessPlain); err == nil {
			token.AccessToken = encAcc
		}
		if encRef, err := EncryptField(cipher, refreshPlain); err == nil {
			token.RefreshToken = encRef
		}
		_ = db.WithContext(ctx).Save(token).Error
	}

	if time.Now().UTC().Before(token.TokenExpiry.Add(-5 * time.Minute)) {
		return accessPlain, nil
	}

	refreshed, err := RefreshAccessToken(ctx, cfg, refreshPlain)
	if err != nil {
		return "", err
	}

	// Salva access_token renovado criptografado
	encAcc, err := EncryptField(cipher, refreshed.AccessToken)
	if err != nil {
		encAcc = refreshed.AccessToken // fallback plain text
	}
	token.AccessToken = encAcc
	token.TokenExpiry = refreshed.Expiry
	_ = db.WithContext(ctx).Save(token).Error

	return refreshed.AccessToken, nil
}

// DecryptField tenta descriptografar o valor. Se falhar (texto puro legado),
// retorna o valor original e sinaliza que estava em texto puro.
// Exportado para uso no handler de OAuth.
func DecryptField(cipher *crypt.Cipher, value string) (plain string, wasPlainText bool) {
	if cipher == nil || value == "" {
		return value, false
	}
	decrypted, err := cipher.Decrypt(value)
	if err != nil {
		return value, true // era texto puro (migração transparente)
	}
	return string(decrypted), false
}

// EncryptField criptografa o valor. Se cipher for nil, retorna o valor sem alteração.
// Exportado para uso no handler de OAuth.
func EncryptField(cipher *crypt.Cipher, value string) (string, error) {
	if cipher == nil || value == "" {
		return value, nil
	}
	return cipher.Encrypt([]byte(value))
}

// EnsureValidTokenPublic é a versão exportada de ensureValidToken — usada pelo
// handler de OAuth para obter um access_token válido sem duplicar a lógica.
func EnsureValidTokenPublic(ctx context.Context, db *gorm.DB, cfg OAuthConfig, cipher *crypt.Cipher, token *models.BarberGoogleToken) (string, error) {
	return ensureValidToken(ctx, db, cfg, cipher, token)
}

func buildEventText(ctx context.Context, db *gorm.DB, ap *models.Appointment) (summary, description string) {
	clientName  := "Cliente"
	serviceName := "Atendimento"

	if ap.Client != nil {
		clientName = ap.Client.Name
	} else if ap.ClientID != nil {
		var c models.Client
		if db.WithContext(ctx).First(&c, *ap.ClientID).Error == nil {
			clientName = c.Name
		}
	}

	if ap.BarberProduct != nil {
		serviceName = ap.BarberProduct.Name
	} else if ap.BarberProductID != nil {
		var s models.BarbershopService
		if db.WithContext(ctx).First(&s, *ap.BarberProductID).Error == nil {
			serviceName = s.Name
		}
	}

	summary = fmt.Sprintf("✂️ %s — %s", serviceName, clientName)

	if ap.Notes != "" {
		description = "Observação: " + ap.Notes
	}

	return
}

func loadTimezone(ctx context.Context, db *gorm.DB, barbershopID uint) string {
	var shop models.Barbershop
	if db.WithContext(ctx).Select("timezone").First(&shop, barbershopID).Error == nil {
		return shop.Timezone
	}
	return "America/Sao_Paulo"
}
