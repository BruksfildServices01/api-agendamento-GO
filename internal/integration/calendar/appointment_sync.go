package calendar

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// SyncAppointmentToGoogle cria um evento no Google Calendar do barbeiro
// de forma assíncrona (best-effort — falhas são apenas logadas).
// tokenGetter é uma função que retorna o access_token válido para o user.
func SyncAppointmentToGoogle(
	db *gorm.DB,
	cfg OAuthConfig,
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

		if err := syncAppointment(ctx, db, cfg, barberID, barbershopID, ap); err != nil {
			log.Printf("[GOOGLE_CAL] sync failed barber=%d appointment=%d: %v",
				barberID, ap.ID, err)
		}
	}()
}

func syncAppointment(
	ctx context.Context,
	db *gorm.DB,
	cfg OAuthConfig,
	barberID uint,
	barbershopID uint,
	ap *models.Appointment,
) error {
	// Carrega token válido do barbeiro
	var token models.BarberGoogleToken
	if err := db.WithContext(ctx).Where("user_id = ?", barberID).First(&token).Error; err != nil {
		return nil // barbeiro não conectou Google — sem erro
	}

	accessToken, err := ensureValidToken(ctx, db, cfg, &token)
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

func ensureValidToken(ctx context.Context, db *gorm.DB, cfg OAuthConfig, token *models.BarberGoogleToken) (string, error) {
	if time.Now().UTC().Before(token.TokenExpiry.Add(-5 * time.Minute)) {
		return token.AccessToken, nil
	}

	refreshed, err := RefreshAccessToken(ctx, cfg, token.RefreshToken)
	if err != nil {
		return "", err
	}

	token.AccessToken = refreshed.AccessToken
	token.TokenExpiry = refreshed.Expiry
	_ = db.WithContext(ctx).Save(token).Error

	return token.AccessToken, nil
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
