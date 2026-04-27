package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// MPOAuthHandler implementa o fluxo OAuth do Mercado Pago.
// O barbeiro clica "Conectar Mercado Pago", é redirecionado para o MP,
// faz login, autoriza o CorteOn e volta com o token automaticamente salvo.
type MPOAuthHandler struct {
	db           *gorm.DB
	clientID     string
	clientSecret string
	callbackURL  string // URL pública do backend para o callback
	appURL       string // URL do frontend para redirecionar após OAuth
	jwtSecret    string // usado para assinar o state
}

func NewMPOAuthHandler(
	db *gorm.DB,
	clientID, clientSecret, callbackURL, appURL, jwtSecret string,
) *MPOAuthHandler {
	return &MPOAuthHandler{
		db:           db,
		clientID:     clientID,
		clientSecret: clientSecret,
		callbackURL:  callbackURL,
		appURL:       strings.TrimRight(appURL, "/"),
		jwtSecret:    jwtSecret,
	}
}

// ── State helpers ─────────────────────────────────────────────────

func (h *MPOAuthHandler) createState(barbershopID uint) string {
	ts   := strconv.FormatInt(time.Now().Unix(), 10)
	data := fmt.Sprintf("%d:%s", barbershopID, ts)
	mac  := hmac.New(sha256.New, []byte(h.jwtSecret))
	mac.Write([]byte(data))
	sig  := hex.EncodeToString(mac.Sum(nil))
	raw  := fmt.Sprintf("%s:%s", data, sig)
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

func (h *MPOAuthHandler) parseState(state string) (uint, error) {
	decoded, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		return 0, errors.New("invalid state encoding")
	}

	parts := strings.SplitN(string(decoded), ":", 3)
	if len(parts) != 3 {
		return 0, errors.New("invalid state format")
	}

	bid, ts, sig := parts[0], parts[1], parts[2]

	// Verifica assinatura HMAC
	data := fmt.Sprintf("%s:%s", bid, ts)
	mac  := hmac.New(sha256.New, []byte(h.jwtSecret))
	mac.Write([]byte(data))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return 0, errors.New("invalid state signature")
	}

	// Verifica validade (10 minutos)
	timestamp, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || time.Now().Unix()-timestamp > 600 {
		return 0, errors.New("state expired")
	}

	id, err := strconv.ParseUint(bid, 10, 64)
	if err != nil {
		return 0, errors.New("invalid barbershop id in state")
	}
	return uint(id), nil
}

// ── Start ─────────────────────────────────────────────────────────

// Start retorna a URL de autorização do Mercado Pago como JSON.
// O frontend faz window.location.href = url para navegar no browser.
// GET /api/me/mercadopago/oauth/start
func (h *MPOAuthHandler) Start(c *gin.Context) {
	if h.clientID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "MP OAuth não configurado"})
		return
	}

	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	state := h.createState(barbershopID)

	authURL := fmt.Sprintf(
		"https://auth.mercadopago.com/authorization?client_id=%s&response_type=code&platform_id=mp&state=%s&redirect_uri=%s",
		url.QueryEscape(h.clientID),
		url.QueryEscape(state),
		url.QueryEscape(h.callbackURL),
	)

	c.JSON(http.StatusOK, gin.H{"url": authURL})
}

// ── Callback ──────────────────────────────────────────────────────

type mpTokenResponse struct {
	AccessToken  string `json:"access_token"`
	PublicKey    string `json:"public_key"`
	RefreshToken string `json:"refresh_token"`
	UserID       int64  `json:"user_id"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	Message      string `json:"message"`
}

// Callback recebe o código do MP e troca por access_token + public_key.
// GET /api/mercadopago/oauth/callback  (rota pública — MP redireciona aqui)
func (h *MPOAuthHandler) Callback(c *gin.Context) {
	redirectBase := h.appURL + "/app/mais/conta"

	code  := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?mp_error=cancelled")
		return
	}

	barbershopID, err := h.parseState(state)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?mp_error=invalid_state")
		return
	}

	tokens, err := h.exchangeCode(c.Request.Context(), code)
	if err != nil || tokens.AccessToken == "" {
		c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?mp_error=exchange_failed")
		return
	}

	// Salva access_token e public_key na config da barbearia
	if err := h.saveTokens(barbershopID, tokens.AccessToken, tokens.PublicKey); err != nil {
		c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?mp_error=save_failed")
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?mp_success=1")
}

func (h *MPOAuthHandler) exchangeCode(ctx interface{ Value(any) any }, code string) (*mpTokenResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"client_id":     h.clientID,
		"client_secret": h.clientSecret,
		"grant_type":    "authorization_code",
		"code":          code,
		"redirect_uri":  h.callbackURL,
	})

	req, err := http.NewRequest(http.MethodPost, "https://api.mercadopago.com/oauth/token", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result mpTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, fmt.Errorf("mp oauth error: %s — %s", result.Error, result.Message)
	}
	return &result, nil
}

func (h *MPOAuthHandler) saveTokens(barbershopID uint, accessToken, publicKey string) error {
	var cfg models.BarbershopPaymentConfig

	err := h.db.Where("barbershop_id = ?", barbershopID).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		cfg = models.BarbershopPaymentConfig{
			BarbershopID:         barbershopID,
			DefaultRequirement:   "none",
			PixExpirationMinutes: 15,
			MPAccessToken:        accessToken,
			MPPublicKey:          publicKey,
			AcceptPix:            true,
		}
		return h.db.Create(&cfg).Error
	}
	if err != nil {
		return err
	}

	cfg.MPAccessToken = accessToken
	if publicKey != "" {
		cfg.MPPublicKey = publicKey
	}
	cfg.AcceptPix = true
	return h.db.Save(&cfg).Error
}

// Status retorna se o OAuth MP está configurado para a barbearia.
// GET /api/me/mercadopago/oauth/status
func (h *MPOAuthHandler) Status(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var cfg models.BarbershopPaymentConfig
	err := h.db.Where("barbershop_id = ?", barbershopID).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"connected": false})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"connected": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"connected":  cfg.MPAccessToken != "" && cfg.MPPublicKey != "",
		"accept_pix": cfg.AcceptPix,
	})
}

// Disconnect remove as credenciais MP da barbearia.
// DELETE /api/me/mercadopago/oauth
func (h *MPOAuthHandler) Disconnect(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	h.db.Model(&models.BarbershopPaymentConfig{}).
		Where("barbershop_id = ?", barbershopID).
		Updates(map[string]any{
			"mp_access_token": "",
			"mp_public_key":   "",
			"accept_pix":      false,
			"accept_credit":   false,
			"accept_debit":    false,
		})

	c.Status(http.StatusNoContent)
}
