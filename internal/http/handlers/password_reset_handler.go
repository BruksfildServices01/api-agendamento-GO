package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// PasswordMailer é satisfeito por EmailNotifier e NoopNotifier.
type PasswordMailer interface {
	SendPasswordReset(ctx context.Context, to, resetLink string) error
}

type PasswordResetHandler struct {
	db     *gorm.DB
	appURL string
	mailer PasswordMailer
}

func NewPasswordResetHandler(db *gorm.DB, appURL string, mailer PasswordMailer) *PasswordResetHandler {
	return &PasswordResetHandler{db: db, appURL: appURL, mailer: mailer}
}

// ======================================================
// POST /auth/password-reset/request
// ======================================================

type passwordResetRequestBody struct {
	Email string `json:"email" binding:"required,email"`
}

func (h *PasswordResetHandler) Request(c *gin.Context) {
	var req passwordResetRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "invalid email")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	ctx := c.Request.Context()

	// Sempre retorna 200 — não revela se o e-mail existe
	var user models.User
	if err := h.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}

	// Gera token seguro
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		httperr.Internal(c, "token_generation_failed", "")
		return
	}
	token := hex.EncodeToString(b)

	// Invalida tokens anteriores não usados do mesmo usuário
	h.db.WithContext(ctx).
		Where("user_id = ? AND used_at IS NULL AND expires_at > ?", user.ID, time.Now()).
		Delete(&models.PasswordResetToken{})

	// Persiste novo token (1 hora de validade)
	prt := models.PasswordResetToken{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := h.db.WithContext(ctx).Create(&prt).Error; err != nil {
		httperr.Internal(c, "failed_to_create_token", "")
		return
	}

	// Envia email (erro não exposto ao cliente)
	resetLink := fmt.Sprintf("%s/redefinir-senha?token=%s", h.appURL, token)
	_ = h.mailer.SendPasswordReset(ctx, email, resetLink)

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ======================================================
// POST /auth/password-reset/confirm
// ======================================================

type passwordResetConfirmBody struct {
	Token    string `json:"token"    binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

func (h *PasswordResetHandler) Confirm(c *gin.Context) {
	var req passwordResetConfirmBody
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "token and password (min 6 chars) are required")
		return
	}

	ctx := c.Request.Context()

	var prt models.PasswordResetToken
	err := h.db.WithContext(ctx).Where("token = ?", req.Token).First(&prt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		httperr.BadRequest(c, "invalid_token", "token not found")
		return
	}
	if err != nil {
		httperr.Internal(c, "db_error", "")
		return
	}

	if prt.UsedAt != nil {
		httperr.BadRequest(c, "token_already_used", "token already used")
		return
	}
	if time.Now().After(prt.ExpiresAt) {
		httperr.BadRequest(c, "token_expired", "token expired")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		httperr.Internal(c, "hash_failed", "")
		return
	}

	now := time.Now()
	txErr := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.User{}).
			Where("id = ?", prt.UserID).
			Update("password_hash", string(hashed)).Error; err != nil {
			return err
		}
		return tx.Model(&prt).Update("used_at", &now).Error
	})

	if txErr != nil {
		httperr.Internal(c, "failed_to_reset_password", "")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
