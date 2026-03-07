package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	infraRepo "github.com/BruksfildServices01/barber-scheduler/internal/infra/repository"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
	"github.com/BruksfildServices01/barber-scheduler/internal/usecase/appointment"
)

////////////////////////////////////////////////////////
// HANDLER
////////////////////////////////////////////////////////

type PublicHandler struct {
	db                *gorm.DB
	createAppointment *appointment.CreatePrivateAppointment
}

func NewPublicHandler(
	db *gorm.DB,
	createAppointment *appointment.CreatePrivateAppointment,
) *PublicHandler {
	return &PublicHandler{
		db:                db,
		createAppointment: createAppointment,
	}
}

////////////////////////////////////////////////////////
// DTOs
////////////////////////////////////////////////////////

type PublicCreateAppointmentRequest struct {
	ClientName  string `json:"client_name" binding:"required"`
	ClientPhone string `json:"client_phone" binding:"required"`
	ClientEmail string `json:"client_email"`
	ProductID   uint   `json:"product_id" binding:"required"`
	Date        string `json:"date" binding:"required"` // YYYY-MM-DD
	Time        string `json:"time" binding:"required"` // HH:mm
	Notes       string `json:"notes"`
}

////////////////////////////////////////////////////////
// PRODUCTS (public catalog of services)
////////////////////////////////////////////////////////

func (h *PublicHandler) ListProducts(c *gin.Context) {
	slug := c.Param("slug")

	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	category := strings.TrimSpace(strings.ToLower(c.Query("category")))
	query := strings.TrimSpace(strings.ToLower(c.Query("query")))

	q := h.db.Where("barbershop_id = ? AND active = true", shop.ID)

	if category != "" {
		q = q.Where("LOWER(category) = ?", category)
	}

	if query != "" {
		like := "%" + query + "%"
		q = q.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", like, like)
	}

	var products []models.BarbershopService
	if err := q.Order("id ASC").Find(&products).Error; err != nil {
		httperr.Internal(c, "failed_to_list_products", "Erro ao listar produtos.")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"barbershop": shop,
		"products":   products,
	})
}

////////////////////////////////////////////////////////
// AVAILABILITY (timezone-safe)
////////////////////////////////////////////////////////

func (h *PublicHandler) AvailabilityForClient(c *gin.Context) {
	slug := c.Param("slug")
	dateStr := strings.TrimSpace(c.Query("date"))
	productIDStr := strings.TrimSpace(c.Query("product_id"))

	if dateStr == "" || productIDStr == "" {
		httperr.BadRequest(c, "missing_params", "Data e serviço obrigatórios.")
		return
	}

	productID, err := strconv.ParseUint(productIDStr, 10, 64)
	if err != nil || productID == 0 {
		httperr.BadRequest(c, "invalid_product_id", "Serviço inválido.")
		return
	}

	// 1) Resolve barbershop
	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	// 2) Barber (owner) — MVP
	var barber models.User
	if err := h.db.
		Where("barbershop_id = ? AND role = ?", shop.ID, "owner").
		First(&barber).Error; err != nil {
		httperr.BadRequest(c, "barber_not_found", "Barbeiro não encontrado.")
		return
	}

	// 3) Date input (YYYY-MM-DD) -> midnight in barbershop timezone (source of truth)
	loc := timezone.Location(shop.Timezone)

	parsed, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		httperr.BadRequest(c, "invalid_date", "Data inválida.")
		return
	}

	date := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, loc)

	// 4) Usecase (timezone-safe internally)
	repo := infraRepo.NewAppointmentGormRepository(h.db)
	uc := appointment.NewGetAvailability(repo)

	slots, err := uc.Execute(
		c.Request.Context(),
		domain.AvailabilityInput{
			BarbershopID: shop.ID,
			BarberID:     barber.ID,
			ProductID:    uint(productID),
			Date:         date,
		},
	)
	if err != nil {
		if httperr.IsBusiness(err, "product_not_found") {
			httperr.BadRequest(c, "product_not_found", "Serviço inválido.")
			return
		}
		httperr.Internal(c, "availability_failed", "Erro ao calcular horários.")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"date":     dateStr,
		"timezone": shop.Timezone,
		"slots":    slots,
	})
}

////////////////////////////////////////////////////////
// CREATE APPOINTMENT (PUBLIC → reuses private usecase)
////////////////////////////////////////////////////////

func (h *PublicHandler) CreateAppointment(c *gin.Context) {
	slug := c.Param("slug")

	// 1) Barbearia
	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	// 2) Request
	var req PublicCreateAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "Dados inválidos.")
		return
	}

	// 3) Barbeiro (owner)
	var barber models.User
	if err := h.db.
		Where("barbershop_id = ? AND role = ?", shop.ID, "owner").
		First(&barber).Error; err != nil {
		httperr.BadRequest(c, "barber_not_found", "Barbeiro não encontrado.")
		return
	}

	// 4) Executa (usecase já é timezone-safe)
	ap, err := h.createAppointment.Execute(
		c.Request.Context(),
		appointment.CreatePrivateAppointmentInput{
			BarbershopID: shop.ID,
			BarberID:     barber.ID,
			ClientName:   req.ClientName,
			ClientPhone:  req.ClientPhone,
			ClientEmail:  req.ClientEmail,
			ProductID:    req.ProductID,
			Date:         req.Date,
			Time:         req.Time,
			Notes:        req.Notes,
		},
	)
	if err != nil {
		// helper já existe no package handlers (appointment_handler.go)
		mapCreateErrors(c, err)
		return
	}

	c.JSON(http.StatusCreated, ap)
}
