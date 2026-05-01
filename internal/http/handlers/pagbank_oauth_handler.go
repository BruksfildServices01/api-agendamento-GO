package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/security/crypt"
	paymentinfra "github.com/BruksfildServices01/barber-scheduler/internal/integration/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/http/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// PagBankOAuthHandler implementa o fluxo OAuth do PagBank.
// O barbeiro clica "Conectar PagBank", é redirecionado ao PagBank,
// autoriza o CorteOn e volta com o token salvo automaticamente.
type PagBankOAuthHandler struct {
	db           *gorm.DB
	clientID     string
	clientSecret string
	callbackURL  string
	appURL       string
	jwtSecret    string
	cipher       *crypt.Cipher
	sandbox      bool
}

func NewPagBankOAuthHandler(
	db *gorm.DB,
	clientID, clientSecret, callbackURL, appURL, jwtSecret string,
	cipher *crypt.Cipher,
	sandbox bool,
) *PagBankOAuthHandler {
	return &PagBankOAuthHandler{
		db:           db,
		clientID:     clientID,
		clientSecret: clientSecret,
		callbackURL:  strings.TrimRight(callbackURL, "/"),
		appURL:       strings.TrimRight(appURL, "/"),
		jwtSecret:    jwtSecret,
		cipher:       cipher,
		sandbox:      sandbox,
	}
}

func (h *PagBankOAuthHandler) authBase() string {
	if h.sandbox {
		return "https://connect.sandbox.pagbank.com.br"
	}
	return "https://connect.pagbank.com.br"
}

func (h *PagBankOAuthHandler) apiBase() string {
	if h.sandbox {
		return "https://sandbox.api.pagseguro.com"
	}
	return "https://api.pagseguro.com"
}

// Start retorna a URL de autorização do PagBank como JSON.
// GET /api/me/pagbank/oauth/start
func (h *PagBankOAuthHandler) Start(c *gin.Context) {
	if h.clientID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "PagBank OAuth não configurado"})
		return
	}

	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	state := createOAuthState(barbershopID, h.jwtSecret)

	authURL := fmt.Sprintf(
		"%s/oauth2/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=%s&state=%s",
		h.authBase(),
		url.QueryEscape(h.clientID),
		url.QueryEscape(h.callbackURL),
		url.QueryEscape("payments.read payments.create"),
		url.QueryEscape(state),
	)

	c.JSON(http.StatusOK, gin.H{"url": authURL})
}

// Callback recebe o código do PagBank e troca por access_token.
// GET /api/pagbank/oauth/callback (rota pública — PagBank redireciona aqui)
func (h *PagBankOAuthHandler) Callback(c *gin.Context) {
	redirectBase := h.appURL + "/app/mais/conta"

	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?pb_error=cancelled")
		return
	}

	barbershopID, err := parseOAuthState(state, h.jwtSecret)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?pb_error=invalid_state")
		return
	}

	accessToken, err := h.exchangeCode(code)
	if err != nil || accessToken == "" {
		c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?pb_error=exchange_failed")
		return
	}

	if err := h.saveProvider(barbershopID, accessToken); err != nil {
		c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?pb_error=save_failed")
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, redirectBase+"?pb_success=1")
}

type pagbankTokenResponse struct {
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
	Message     string `json:"message"`
}

func (h *PagBankOAuthHandler) exchangeCode(code string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     h.clientID,
		"client_secret": h.clientSecret,
		"code":          code,
		"redirect_uri":  h.callbackURL,
	})

	req, err := http.NewRequest(http.MethodPost, h.apiBase()+"/oauth2/token", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("pagbank token exchange: %w", err)
	}
	defer resp.Body.Close()

	var result pagbankTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("pagbank oauth: %s — %s", result.Error, result.Message)
	}
	return result.AccessToken, nil
}

func (h *PagBankOAuthHandler) saveProvider(barbershopID uint, accessToken string) error {
	if h.cipher == nil {
		return fmt.Errorf("cipher não configurado — PAYMENT_CREDENTIALS_ENCRYPTION_KEY ausente")
	}

	raw, err := json.Marshal(paymentinfra.PagBankCredentials{AccessToken: accessToken})
	if err != nil {
		return err
	}
	encrypted, err := h.cipher.Encrypt(raw)
	if err != nil {
		return err
	}
	// Zera plaintext imediatamente
	for i := range raw {
		raw[i] = 0
	}

	env := "production"
	if h.sandbox {
		env = "sandbox"
	}

	return h.db.Exec(`
		INSERT INTO barbershop_payment_providers
			(barbershop_id, provider, enabled, environment, credentials_encrypted, created_at, updated_at)
		VALUES (?, 'pagbank', true, ?, ?, NOW(), NOW())
		ON CONFLICT (barbershop_id, provider) DO UPDATE SET
			credentials_encrypted = EXCLUDED.credentials_encrypted,
			enabled               = true,
			environment           = EXCLUDED.environment,
			updated_at            = NOW()
	`, barbershopID, env, encrypted).Error
}

// Status retorna se o PagBank está conectado para a barbearia.
// GET /api/me/pagbank/oauth/status
func (h *PagBankOAuthHandler) Status(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var p models.BarbershopPaymentProvider
	err := h.db.Where("barbershop_id = ? AND provider = ?", barbershopID, "pagbank").First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"connected": false})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"connected": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"connected":   p.Enabled && p.CredentialsEncrypted != nil,
		"environment": p.Environment,
	})
}

// Disconnect remove as credenciais PagBank da barbearia.
// DELETE /api/me/pagbank/oauth
func (h *PagBankOAuthHandler) Disconnect(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	result := h.db.WithContext(c.Request.Context()).Exec(
		`UPDATE barbershop_payment_providers
		 SET credentials_encrypted = NULL, enabled = false, updated_at = NOW()
		 WHERE barbershop_id = ? AND provider = 'pagbank'`,
		barbershopID,
	)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to disconnect"})
		return
	}

	c.Status(http.StatusNoContent)
}
