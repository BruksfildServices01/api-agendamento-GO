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

// --------- Requests ---------

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

// --------- Handlers ---------

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"details": err.Error(),
		})
		return
	}

	// --------------------------------------------------
	// Slug
	// --------------------------------------------------
	slug := strings.ToLower(strings.TrimSpace(req.BarbershopSlug))

	var count int64
	h.db.Model(&models.Barbershop{}).
		Where("slug = ?", slug).
		Count(&count)

	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "slug_already_exists",
		})
		return
	}

	// --------------------------------------------------
	// Barbearia
	// --------------------------------------------------
	shop := models.Barbershop{
		Name:     req.BarbershopName,
		Slug:     slug,
		Phone:    req.BarbershopPhone,
		Address:  req.BarbershopAddress,
		Timezone: "America/Sao_Paulo",
	}

	if err := h.db.Create(&shop).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed_to_create_barbershop",
		})
		return
	}

	// --------------------------------------------------
	// Usuário (owner)
	// --------------------------------------------------
	hashed, err := bcrypt.GenerateFromPassword(
		[]byte(req.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed_to_hash_password",
		})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))

	if !validators.IsEmailDomainValid(email) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_email_domain",
			"message": "O domínio do e-mail informado não parece ser válido.",
		})
		return
	}

	user := models.User{
		BarbershopID: shop.ID,
		Name:         req.Name,
		Email:        email,
		PasswordHash: string(hashed),
		Phone:        req.Phone,
		Role:         "owner",
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed_to_create_user",
		})
		return
	}

	// --------------------------------------------------
	// ⏰ Horários padrão (Seg–Sex 09–17)
	// --------------------------------------------------
	var workingHours []models.WorkingHours

	for weekday := 0; weekday <= 6; weekday++ {
		active := weekday >= 1 && weekday <= 5

		wh := models.WorkingHours{
			BarberID:     user.ID,
			BarbershopID: shop.ID, // Associando à barbearia
			Weekday:      weekday,
			Active:       active,
		}

		if active {
			wh.StartTime = "09:00"
			wh.EndTime = "17:00"
		}

		workingHours = append(workingHours, wh)
	}

	if err := h.db.Create(&workingHours).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed_to_create_working_hours",
		})
		return
	}

	// --------------------------------------------------
	// ✂️ SERVIÇO PADRÃO (MVP)
	// --------------------------------------------------
	defaultProduct := models.BarberProduct{
		BarbershopID: shop.ID,
		Name:         "Corte de cabelo",
		Description:  "Corte masculino tradicional",
		DurationMin:  30,
		Price:        50.00,
		Active:       true,
		Category:     "corte",
	}

	if err := h.db.Create(&defaultProduct).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed_to_create_default_product",
		})
		return
	}

	// --------------------------------------------------
	// Token
	// --------------------------------------------------
	token, err := h.generateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed_to_generate_token",
		})
		return
	}

	// --------------------------------------------------
	// Response
	// --------------------------------------------------
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
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"details": err.Error(),
		})
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

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
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

// Cria os horários padrão para o barbeiro
func createDefaultWorkingHours(db *gorm.DB, barberID uint, barbershopID uint) error {
	workingHours := []models.WorkingHours{
		{BarberID: barberID, Weekday: 1, StartTime: "09:00", EndTime: "17:00", Active: true}, // Segunda-feira
		{BarberID: barberID, Weekday: 2, StartTime: "09:00", EndTime: "17:00", Active: true}, // Terça-feira
		{BarberID: barberID, Weekday: 3, StartTime: "09:00", EndTime: "17:00", Active: true}, // Quarta-feira
		{BarberID: barberID, Weekday: 4, StartTime: "09:00", EndTime: "17:00", Active: true}, // Quinta-feira
		{BarberID: barberID, Weekday: 5, StartTime: "09:00", EndTime: "17:00", Active: true}, // Sexta-feira
		{BarberID: barberID, Weekday: 6, Active: false},                                      // Sábado inativo
		{BarberID: barberID, Weekday: 7, Active: false},                                      // Domingo inativo
	}

	return db.Create(&workingHours).Error
}

// --------- JWT ---------

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
