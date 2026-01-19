package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
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
	db    *gorm.DB
	audit *audit.Dispatcher
}

func NewPublicHandler(db *gorm.DB) *PublicHandler {
	logger := audit.New(db)
	dispatcher := audit.NewDispatcher(logger)

	return &PublicHandler{
		db:    db,
		audit: dispatcher,
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
// PRODUCTS
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

	q := h.db.
		Where("barbershop_id = ? AND active = true", shop.ID)

	if category != "" {
		q = q.Where("LOWER(category) = ?", category)
	}

	if query != "" {
		like := "%" + query + "%"
		q = q.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", like, like)
	}

	var products []models.BarberProduct
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
// AVAILABILITY (REUSO TOTAL DO USE CASE)
////////////////////////////////////////////////////////

func (h *PublicHandler) AvailabilityForClient(c *gin.Context) {
	slug := c.Param("slug")
	dateStr := c.Query("date")
	productIDStr := c.Query("product_id")

	if dateStr == "" || productIDStr == "" {
		httperr.BadRequest(c, "missing_params", "Data e serviço obrigatórios.")
		return
	}

	productID, err := strconv.ParseUint(productIDStr, 10, 64)
	if err != nil {
		httperr.BadRequest(c, "invalid_product_id", "Serviço inválido.")
		return
	}

	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	var barber models.User
	if err := h.db.
		Where("barbershop_id = ? AND role = ?", shop.ID, "owner").
		First(&barber).Error; err != nil {

		httperr.BadRequest(c, "barber_not_found", "Barbeiro não encontrado.")
		return
	}

	date, err := time.ParseInLocation(
		"2006-01-02",
		dateStr,
		timezone.Location(shop.Timezone),
	)
	if err != nil {
		httperr.BadRequest(c, "invalid_date", "Data inválida.")
		return
	}

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
		"date":  dateStr,
		"slots": slots,
	})
}

////////////////////////////////////////////////////////
// CREATE APPOINTMENT (PUBLIC → REUSA PRIVATE)
////////////////////////////////////////////////////////

func (h *PublicHandler) CreateAppointment(c *gin.Context) {
	slug := c.Param("slug")

	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	var req PublicCreateAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "Dados inválidos.")
		return
	}

	var barber models.User
	if err := h.db.
		Where("barbershop_id = ? AND role = ?", shop.ID, "owner").
		First(&barber).Error; err != nil {

		httperr.BadRequest(c, "barber_not_found", "Barbeiro não encontrado.")
		return
	}

	repo := infraRepo.NewAppointmentGormRepository(h.db)
	uc := appointment.NewCreatePrivateAppointment(repo, h.audit)

	ap, err := uc.Execute(
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
		mapCreateErrors(c, err)
		return
	}

	c.JSON(http.StatusCreated, ap)
}
