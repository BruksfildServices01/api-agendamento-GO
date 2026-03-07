package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/paymentconfig"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/BruksfildServices01/barber-scheduler/internal/validators"
)

type AuthHandler struct {
	db     *gorm.DB
	config *config.Config
}

func NewAuthHandler(db *gorm.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{db: db, config: cfg}
}

// ======================================================
// REQUESTS
// ======================================================

type RegisterRequest struct {
	BarbershopName    string `json:"barbershop_name" binding:"required"`
	BarbershopSlug    string `json:"barbershop_slug" binding:"required"`
	BarbershopPhone   string `json:"barbershop_phone"`
	BarbershopAddress string `json:"barbershop_address"`

	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Phone    string `json:"phone"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// ======================================================
// REGISTER (TRANSACTIONAL, CORRETO)
// ======================================================

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}

	slug := strings.ToLower(strings.TrimSpace(req.BarbershopSlug))

	// --------------------------------------------------
	// Validação de slug (antes da transação)
	// --------------------------------------------------
	var count int64
	h.db.Model(&models.Barbershop{}).
		Where("slug = ?", slug).
		Count(&count)

	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug_already_exists"})
		return
	}

	ctx := c.Request.Context()

	// ==================================================
	// TRANSACTION
	// ==================================================
	err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		// -------------------------------
		// Barbearia
		// -------------------------------
		shop := models.Barbershop{
			Name:     req.BarbershopName,
			Slug:     slug,
			Phone:    req.BarbershopPhone,
			Address:  req.BarbershopAddress,
			Timezone: "America/Sao_Paulo",
		}

		if err := tx.Create(&shop).Error; err != nil {
			return err
		}

		// -------------------------------
		// 💳 Payment Config DEFAULT
		// -------------------------------
		paymentCfg := domainPayment.Default(shop.ID)

		if err := tx.Create(&models.BarbershopPaymentConfig{
			BarbershopID:         shop.ID,
			DefaultRequirement:   models.PaymentRequirement(paymentCfg.DefaultRequirement),
			PixExpirationMinutes: paymentCfg.PixExpirationMinutes,
		}).Error; err != nil {
			return err
		}

		// -------------------------------
		// Usuário OWNER
		// -------------------------------
		hashed, err := bcrypt.GenerateFromPassword(
			[]byte(req.Password),
			bcrypt.DefaultCost,
		)
		if err != nil {
			return err
		}

		email := strings.ToLower(strings.TrimSpace(req.Email))
		if !validators.IsEmailDomainValid(email) {
			return gorm.ErrInvalidData
		}

		user := models.User{
			BarbershopID: &shop.ID,
			Name:         req.Name,
			Email:        email,
			PasswordHash: string(hashed),
			Phone:        req.Phone,
			Role:         "owner",
		}

		if err := tx.Create(&user).Error; err != nil {
			return err
		}

		// -------------------------------
		// Horários padrão
		// -------------------------------
		var workingHours []models.WorkingHours

		for weekday := 0; weekday <= 6; weekday++ {
			active := weekday >= 1 && weekday <= 5

			wh := models.WorkingHours{
				BarbershopID: shop.ID,
				BarberID:     user.ID,
				Weekday:      weekday,
				Active:       active,
			}

			if active {
				wh.StartTime = "09:00"
				wh.EndTime = "17:00"
			}

			workingHours = append(workingHours, wh)
		}

		if err := tx.Create(&workingHours).Error; err != nil {
			return err
		}

		// -------------------------------
		// Serviço padrão
		// -------------------------------
		defaultProduct := models.BarbershopService{
			BarbershopID: &shop.ID,
			Name:         "Corte de cabelo",
			Description:  "Corte masculino tradicional",
			DurationMin:  30,
			Price:        5000,
			Active:       true,
			Category:     "corte",
		}

		if err := tx.Create(&defaultProduct).Error; err != nil {
			return err
		}

		// -------------------------------
		// RESPONSE FINAL
		// -------------------------------
		token, err := h.generateToken(&user)
		if err != nil {
			return err
		}

		c.JSON(http.StatusCreated, gin.H{
			"user": gin.H{
				"id":            user.ID,
				"name":          user.Name,
				"email":         user.Email,
				"phone":         user.Phone,
				"barbershop_id": user.BarbershopID,
			},
			"barbershop": gin.H{
				"id":      shop.ID,
				"name":    shop.Name,
				"slug":    shop.Slug,
				"phone":   shop.Phone,
				"address": shop.Address,
			},
			"token": token,
		})

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed_to_register",
		})
	}
}

// ======================================================
// JWT
// ======================================================

func (h *AuthHandler) generateToken(user *models.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":          user.ID,
		"barbershopId": user.BarbershopID,
		"role":         user.Role,
		"exp":          time.Now().Add(24 * time.Hour).Unix(),
		"iat":          time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.config.JWTSecret))
}

// ======================================================
// LOGIN
// ======================================================

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))

	var user models.User
	if err := h.db.Preload("Barbershop").
		Where("email = ?", email).
		First(&user).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(req.Password),
	); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials"})
		return
	}

	token, err := h.generateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_generate_token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":            user.ID,
			"name":          user.Name,
			"email":         user.Email,
			"phone":         user.Phone,
			"barbershop_id": user.BarbershopID,
		},
		"barbershop": gin.H{
			"id":      user.Barbershop.ID,
			"name":    user.Barbershop.Name,
			"slug":    user.Barbershop.Slug,
			"phone":   user.Barbershop.Phone,
			"address": user.Barbershop.Address,
		},
		"token": token,
	})
}
