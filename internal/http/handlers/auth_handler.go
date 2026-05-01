package handlers

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/paymentconfig"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
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
	BarbershopAddress string `json:"barbershop_address"` // legado — mantido para retrocompatibilidade

	// Endereço estruturado (novo)
	BarbershopCEP          string `json:"barbershop_cep"`
	BarbershopStreetName   string `json:"barbershop_street_name"`
	BarbershopStreetNumber string `json:"barbershop_street_number"`
	BarbershopComplement   string `json:"barbershop_complement"`
	BarbershopNeighborhood string `json:"barbershop_neighborhood"`
	BarbershopCity         string `json:"barbershop_city"`
	BarbershopState        string `json:"barbershop_state"`

	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Phone    string `json:"phone"`
}

// composeAddress monta a string de endereço para exibição/Google Maps.
func composeAddress(streetName, streetNumber, complement, neighborhood, city, state, cep string) string {
	var parts []string

	street := strings.TrimSpace(streetName)
	num := strings.TrimSpace(streetNumber)
	if street != "" {
		if num != "" {
			parts = append(parts, street+", "+num)
		} else {
			parts = append(parts, street)
		}
	}

	if comp := strings.TrimSpace(complement); comp != "" {
		parts = append(parts, comp)
	}
	if neigh := strings.TrimSpace(neighborhood); neigh != "" {
		parts = append(parts, neigh)
	}

	c, s := strings.TrimSpace(city), strings.TrimSpace(state)
	switch {
	case c != "" && s != "":
		parts = append(parts, c+" - "+s)
	case c != "":
		parts = append(parts, c)
	case s != "":
		parts = append(parts, s)
	}

	if cp := strings.TrimSpace(cep); cp != "" {
		parts = append(parts, cp)
	}

	return strings.Join(parts, ", ")
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
		httperr.BadRequest(c, "invalid_request", "invalid_request")
		return
	}

	slug := uniqueSlug(h.db, strings.ToLower(strings.TrimSpace(req.BarbershopSlug)))

	ctx := c.Request.Context()

	var (
		createdShop   models.Barbershop
		createdUser   models.User
		responseToken string
	)

	trialEnd := time.Now().AddDate(0, 0, h.config.TrialDays)

	// ==================================================
	// TRANSACTION
	// ==================================================
	err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// -------------------------------
		// Barbearia
		// -------------------------------
		cep := strings.TrimSpace(req.BarbershopCEP)
		streetName := strings.TrimSpace(req.BarbershopStreetName)
		streetNumber := strings.TrimSpace(req.BarbershopStreetNumber)
		complement := strings.TrimSpace(req.BarbershopComplement)
		neighborhood := strings.TrimSpace(req.BarbershopNeighborhood)
		city := strings.TrimSpace(req.BarbershopCity)
		state := strings.TrimSpace(req.BarbershopState)

		// Monta a string de exibição: prefere campos estruturados, cai para legado
		address := strings.TrimSpace(req.BarbershopAddress)
		if streetName != "" || city != "" {
			address = composeAddress(streetName, streetNumber, complement, neighborhood, city, state, cep)
		}

		shop := models.Barbershop{
			Name:         req.BarbershopName,
			Slug:         slug,
			Phone:        req.BarbershopPhone,
			Address:      address,
			CEP:          cep,
			StreetName:   streetName,
			StreetNumber: streetNumber,
			Complement:   complement,
			Neighborhood: neighborhood,
			City:         city,
			State:        state,
			Email:        strings.ToLower(strings.TrimSpace(req.Email)),
			Timezone:     "America/Sao_Paulo",
			Status:       "trial",
			TrialEndsAt:  &trialEnd,
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
			AcceptCash:           paymentCfg.AcceptCash,
			AcceptPix:            paymentCfg.AcceptPix,
			AcceptCredit:         paymentCfg.AcceptCredit,
			AcceptDebit:          paymentCfg.AcceptDebit,
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
		// Dados de exemplo (onboarding)
		// -------------------------------

		// Serviços
		seedServices := []models.BarbershopService{
			{BarbershopID: shop.ID, Name: "Corte Masculino", Description: "Corte masculino tradicional", DurationMin: 30, Price: 4500, Active: true, Category: "corte"},
			{BarbershopID: shop.ID, Name: "Barba", Description: "Modelagem e alinhamento de barba", DurationMin: 20, Price: 3000, Active: true, Category: "barba"},
			{BarbershopID: shop.ID, Name: "Corte + Barba", Description: "Combo completo: corte e barba", DurationMin: 50, Price: 7000, Active: true, Category: "combo"},
			{BarbershopID: shop.ID, Name: "Degradê", Description: "Corte estilo degradê com acabamento", DurationMin: 40, Price: 5000, Active: true, Category: "corte"},
		}
		if err := tx.Create(&seedServices).Error; err != nil {
			return err
		}

		// Produtos
		seedProducts := []models.Product{
			{BarbershopID: shop.ID, Name: "Pomada Modeladora", Description: "Modeladora para finalizar o visual", Category: "finalizadores", Price: 3500, Stock: 10, Active: true},
			{BarbershopID: shop.ID, Name: "Shampoo Masculino", Description: "Shampoo específico para cabelos masculinos", Category: "higiene", Price: 2500, Stock: 5, Active: true},
		}
		if err := tx.Create(&seedProducts).Error; err != nil {
			return err
		}

		// Sugestão: Corte Masculino → Pomada Modeladora
		seedSuggestion := models.ServiceSuggestedProduct{
			BarbershopID: shop.ID,
			ServiceID:    seedServices[0].ID,
			ProductID:    seedProducts[0].ID,
			Active:       true,
		}
		if err := tx.Create(&seedSuggestion).Error; err != nil {
			return err
		}

		// Plano de assinatura de exemplo
		seedPlan := models.Plan{
			BarbershopID:      shop.ID,
			Name:              "Plano Mensal",
			MonthlyPriceCents: 12000,
			DurationDays:      30,
			CutsIncluded:      4,
			DiscountPercent:   10,
			Active:            true,
		}
		if err := tx.Create(&seedPlan).Error; err != nil {
			return err
		}

		// Vincula o plano aos serviços de corte e combo
		for _, svc := range seedServices {
			if svc.Category == "corte" || svc.Category == "combo" {
				if err := tx.Exec(
					`INSERT INTO plan_services (plan_id, service_id) VALUES (?, ?)`,
					seedPlan.ID, svc.ID,
				).Error; err != nil {
					return err
				}
			}
		}

		token, err := h.generateToken(&user)
		if err != nil {
			return err
		}

		createdShop = shop
		createdUser = user
		responseToken = token

		return nil
	})

	if err != nil {
		if errors.Is(err, gorm.ErrInvalidData) {
			httperr.BadRequest(c, "invalid_email_domain", "invalid_email_domain")
			return
		}

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			httperr.BadRequest(c, "email_already_registered", "email_already_registered")
			return
		}

		httperr.Internal(c, "failed_to_register", "failed_to_register")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user": gin.H{
			"id":            createdUser.ID,
			"name":          createdUser.Name,
			"email":         createdUser.Email,
			"phone":         createdUser.Phone,
			"barbershop_id": createdUser.BarbershopID,
		},
		"barbershop": gin.H{
			"id":      createdShop.ID,
			"name":    createdShop.Name,
			"slug":    createdShop.Slug,
			"phone":   createdShop.Phone,
			"address": createdShop.Address,
		},
		"token": responseToken,
	})
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
		httperr.BadRequest(c, "invalid_request", "invalid_request")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))

	var user models.User
	if err := h.db.Preload("Barbershop").
		Where("email = ?", email).
		First(&user).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			httperr.Unauthorized(c, "invalid_credentials", "invalid_credentials")
			return
		}

		httperr.Internal(c, "internal_error", "internal_error")
		return
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(req.Password),
	); err != nil {
		httperr.Unauthorized(c, "invalid_credentials", "invalid_credentials")
		return
	}

	token, err := h.generateToken(&user)
	if err != nil {
		httperr.Internal(c, "failed_to_generate_token", "failed_to_generate_token")
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

// uniqueSlug garante que o slug seja único no banco.
// Se o slug base já existe, adiciona um sufixo aleatório de 4 dígitos.
func uniqueSlug(db *gorm.DB, base string) string {
	if base == "" {
		base = "barbearia"
	}
	candidate := base
	for i := 0; i < 10; i++ {
		var count int64
		db.Model(&models.Barbershop{}).Where("slug = ?", candidate).Count(&count)
		if count == 0 {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%04d", base, rand.Intn(9000)+1000)
	}
	// fallback com timestamp — impossível colidir
	return fmt.Sprintf("%s-%d", base, time.Now().UnixMilli()%100000)
}
