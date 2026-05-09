package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	gcal "github.com/BruksfildServices01/barber-scheduler/internal/integration/calendar"
	"github.com/BruksfildServices01/barber-scheduler/internal/http/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// GoogleOAuthHandler implementa o fluxo OAuth do Google Calendar.
// Cada barbeiro conecta sua própria conta Google individualmente.
type GoogleOAuthHandler struct {
	db        *gorm.DB
	oauthCfg  gcal.OAuthConfig
	appURL    string
	jwtSecret string
}

func NewGoogleOAuthHandler(db *gorm.DB, cfg *config.Config) *GoogleOAuthHandler {
	return &GoogleOAuthHandler{
		db: db,
		oauthCfg: gcal.OAuthConfig{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
		},
		appURL:    cfg.AppURL,
		jwtSecret: cfg.JWTSecret,
	}
}

// Start retorna a URL de autorização do Google.
// GET /api/me/google/oauth/start
func (h *GoogleOAuthHandler) Start(c *gin.Context) {
	if h.oauthCfg.ClientID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Google OAuth não configurado"})
		return
	}

	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	state := createOAuthState(barbershopID, h.jwtSecret)
	authURL := gcal.AuthURL(h.oauthCfg, state)

	c.JSON(http.StatusOK, gin.H{"url": authURL})
}

// Callback recebe o code do Google e troca pelo token.
// GET /api/google/oauth/callback (rota pública — Google redireciona aqui)
func (h *GoogleOAuthHandler) Callback(c *gin.Context) {
	redirectBase := h.appURL + "/app/mais/conta"

	code  := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?google_error=cancelled")
		return
	}

	barbershopID, err := parseOAuthState(state, h.jwtSecret)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?google_error=invalid_state")
		return
	}

	// Encontra o user_id do owner/barber logado na barbearia
	// (quem iniciou o OAuth é o usuário cujo barbershop_id e que fez o start)
	var user models.User
	if err := h.db.WithContext(c.Request.Context()).
		Where("barbershop_id = ? AND role = 'owner'", barbershopID).
		First(&user).Error; err != nil {
		c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?google_error=user_not_found")
		return
	}

	token, err := gcal.ExchangeCode(c.Request.Context(), h.oauthCfg, code)
	if err != nil || token.AccessToken == "" {
		log.Printf("[GOOGLE] exchange error: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?google_error=exchange_failed")
		return
	}

	if err := h.saveToken(c.Request.Context(), user.ID, barbershopID, token); err != nil {
		log.Printf("[GOOGLE] save token error: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?google_error=save_failed")
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?google_success=1")
}

// Status retorna se o Google Calendar está conectado para o usuário.
// GET /api/me/google/oauth/status
func (h *GoogleOAuthHandler) Status(c *gin.Context) {
	userID       := c.MustGet(middleware.ContextUserID).(uint)
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var token models.BarberGoogleToken
	err := h.db.WithContext(c.Request.Context()).
		Where("user_id = ? AND barbershop_id = ?", userID, barbershopID).
		First(&token).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"connected": false})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"connected": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"connected": true,
	})
}

// Disconnect remove o token do Google Calendar do barbeiro.
// DELETE /api/me/google/oauth
func (h *GoogleOAuthHandler) Disconnect(c *gin.Context) {
	userID       := c.MustGet(middleware.ContextUserID).(uint)
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	result := h.db.WithContext(c.Request.Context()).
		Where("user_id = ? AND barbershop_id = ?", userID, barbershopID).
		Delete(&models.BarberGoogleToken{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to disconnect"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *GoogleOAuthHandler) saveToken(
	ctx context.Context,
	userID uint,
	barbershopID uint,
	token *gcal.TokenResult,
) error {
	record := models.BarberGoogleToken{
		UserID:       userID,
		BarbershopID: barbershopID,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenExpiry:  token.Expiry,
	}

	var existing models.BarberGoogleToken
	err := h.db.WithContext(ctx).Where("user_id = ?", userID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return h.db.WithContext(ctx).Create(&record).Error
	}
	if err != nil {
		return err
	}

	existing.AccessToken  = token.AccessToken
	existing.RefreshToken = token.RefreshToken
	existing.TokenExpiry  = token.Expiry
	return h.db.WithContext(ctx).Save(&existing).Error
}

// GetValidToken retorna um access_token válido para o usuário,
// renovando via refresh_token se necessário.
// Retorna ("", nil) se o usuário não tiver Google Calendar conectado.
func GetValidToken(ctx context.Context, db *gorm.DB, cfg gcal.OAuthConfig, userID uint) (string, error) {
	var token models.BarberGoogleToken
	err := db.WithContext(ctx).Where("user_id = ?", userID).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil // não conectado — sem erro
	}
	if err != nil {
		return "", fmt.Errorf("google token load: %w", err)
	}

	// Token ainda válido (com margem de 5 minutos)
	if time.Now().UTC().Before(token.TokenExpiry.Add(-5 * time.Minute)) {
		return token.AccessToken, nil
	}

	// Renova o token
	refreshed, err := gcal.RefreshAccessToken(ctx, cfg, token.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("google token refresh: %w", err)
	}

	token.AccessToken = refreshed.AccessToken
	token.TokenExpiry = refreshed.Expiry
	if err := db.WithContext(ctx).Save(&token).Error; err != nil {
		log.Printf("[GOOGLE] failed to save refreshed token for user %d: %v", userID, err)
	}

	return token.AccessToken, nil
}
