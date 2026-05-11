package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	gcal "github.com/BruksfildServices01/barber-scheduler/internal/integration/calendar"
	"github.com/BruksfildServices01/barber-scheduler/internal/http/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/BruksfildServices01/barber-scheduler/internal/security/crypt"
)

// GoogleOAuthHandler implementa o fluxo OAuth do Google Calendar.
// Cada barbeiro conecta sua própria conta Google individualmente.
type GoogleOAuthHandler struct {
	db        *gorm.DB
	oauthCfg  gcal.OAuthConfig
	appURL    string
	jwtSecret string
	cipher    *crypt.Cipher // nil em dev sem PAYMENT_CREDENTIALS_ENCRYPTION_KEY
}

func NewGoogleOAuthHandler(db *gorm.DB, cfg *config.Config, cipher *crypt.Cipher) *GoogleOAuthHandler {
	return &GoogleOAuthHandler{
		db: db,
		oauthCfg: gcal.OAuthConfig{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
		},
		appURL:    cfg.AppURL,
		jwtSecret: cfg.JWTSecret,
		cipher:    cipher,
	}
}

// createGoogleOAuthState cria um state HMAC que carrega tanto barbershopID quanto userID.
// Formato: base64url(barbershopID:userID:timestamp:HMAC_SHA256)
func createGoogleOAuthState(barbershopID, userID uint, secret string) string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	data := fmt.Sprintf("%d:%d:%s", barbershopID, userID, ts)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	sig := hex.EncodeToString(mac.Sum(nil))
	raw := fmt.Sprintf("%s:%s", data, sig)
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

// parseGoogleOAuthState valida e extrai barbershopID + userID do state.
func parseGoogleOAuthState(state, secret string) (barbershopID, userID uint, err error) {
	decoded, decErr := base64.URLEncoding.DecodeString(state)
	if decErr != nil {
		return 0, 0, errors.New("invalid state encoding")
	}
	// formato: barbershopID:userID:timestamp:sig
	parts := strings.SplitN(string(decoded), ":", 4)
	if len(parts) != 4 {
		return 0, 0, errors.New("invalid state format")
	}
	bid, uid, ts, sig := parts[0], parts[1], parts[2], parts[3]
	data := fmt.Sprintf("%s:%s:%s", bid, uid, ts)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return 0, 0, errors.New("invalid state signature")
	}
	timestamp, tsErr := strconv.ParseInt(ts, 10, 64)
	if tsErr != nil || time.Now().Unix()-timestamp > 600 {
		return 0, 0, errors.New("state expired")
	}
	b, _ := strconv.ParseUint(bid, 10, 64)
	u, _ := strconv.ParseUint(uid, 10, 64)
	return uint(b), uint(u), nil
}

// Start retorna a URL de autorização do Google.
// GET /api/me/google/oauth/start
func (h *GoogleOAuthHandler) Start(c *gin.Context) {
	if h.oauthCfg.ClientID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Google OAuth não configurado"})
		return
	}

	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	userID := c.MustGet(middleware.ContextUserID).(uint)
	// State carrega barbershopID + userID para garantir que o callback
	// salve o token no usuário correto, seja ele owner ou barber.
	state := createGoogleOAuthState(barbershopID, userID, h.jwtSecret)
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

	barbershopID, userID, err := parseGoogleOAuthState(state, h.jwtSecret)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?google_error=invalid_state")
		return
	}

	// Valida que o usuário existe na barbearia indicada
	var user models.User
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND barbershop_id = ?", userID, barbershopID).
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
	// Criptografa access_token e refresh_token antes de persistir.
	// Se cipher for nil (dev sem chave configurada), salva em texto puro.
	encAccess, err := gcal.EncryptField(h.cipher, token.AccessToken)
	if err != nil {
		return fmt.Errorf("encrypt access_token: %w", err)
	}
	encRefresh, err := gcal.EncryptField(h.cipher, token.RefreshToken)
	if err != nil {
		return fmt.Errorf("encrypt refresh_token: %w", err)
	}

	record := models.BarberGoogleToken{
		UserID:       userID,
		BarbershopID: barbershopID,
		AccessToken:  encAccess,
		RefreshToken: encRefresh,
		TokenExpiry:  token.Expiry,
	}

	var existing models.BarberGoogleToken
	err = h.db.WithContext(ctx).Where("user_id = ?", userID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return h.db.WithContext(ctx).Create(&record).Error
	}
	if err != nil {
		return err
	}

	existing.AccessToken  = encAccess
	existing.RefreshToken = encRefresh
	existing.TokenExpiry  = token.Expiry
	return h.db.WithContext(ctx).Save(&existing).Error
}

// GetValidToken retorna um access_token válido para o usuário,
// renovando via refresh_token se necessário.
// Retorna ("", nil) se o usuário não tiver Google Calendar conectado.
// cipher é usado para descriptografar os tokens; nil = sem criptografia (dev).
func GetValidToken(ctx context.Context, db *gorm.DB, cfg gcal.OAuthConfig, cipher *crypt.Cipher, userID uint) (string, error) {
	var token models.BarberGoogleToken
	err := db.WithContext(ctx).Where("user_id = ?", userID).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil // não conectado — sem erro
	}
	if err != nil {
		return "", fmt.Errorf("google token load: %w", err)
	}

	// Delega para ensureValidToken do pacote calendar que cuida de
	// descriptografia, backward compat com tokens antigos e refresh.
	return gcal.EnsureValidTokenPublic(ctx, db, cfg, cipher, &token)
}
