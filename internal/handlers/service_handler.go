package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	domainService "github.com/BruksfildServices01/barber-scheduler/internal/domain/service"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	serviceUC "github.com/BruksfildServices01/barber-scheduler/internal/usecase/service"
)

type ServiceHandler struct {
	db       *gorm.DB
	createUC *serviceUC.CreateService
	updateUC *serviceUC.UpdateService
}

func NewServiceHandler(
	db *gorm.DB,
	createUC *serviceUC.CreateService,
	updateUC *serviceUC.UpdateService,
) *ServiceHandler {
	return &ServiceHandler{
		db:       db,
		createUC: createUC,
		updateUC: updateUC,
	}
}

//
// ======================================================
// REQUEST DTOs (JSON usa CENTAVOS)
// ======================================================
//

type CreateServiceRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	DurationMin int    `json:"duration_min" binding:"required,min=1"`
	Price       int64  `json:"price" binding:"required"` // cents
	Category    string `json:"category"`
}

type UpdateServiceRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	DurationMin *int    `json:"duration_min,omitempty"`
	Price       *int64  `json:"price,omitempty"` // cents
	Active      *bool   `json:"active,omitempty"`
	Category    *string `json:"category,omitempty"`
}

//
// ======================================================
// LIST
// ======================================================
//
// Conservador: mantém listagem atual via GORM para preservar
// filtros já existentes nesta fase da sprint.
//

func (h *ServiceHandler) List(c *gin.Context) {
	barbershopIDVal, ok := c.Get(middleware.ContextBarbershopID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_context"})
		return
	}
	barbershopID := barbershopIDVal.(uint)

	category := strings.ToLower(strings.TrimSpace(c.Query("category")))
	activeStr := strings.TrimSpace(c.Query("active"))
	query := strings.ToLower(strings.TrimSpace(c.Query("query")))
	minPriceStr := strings.TrimSpace(c.Query("min_price"))
	maxPriceStr := strings.TrimSpace(c.Query("max_price"))
	minDurationStr := strings.TrimSpace(c.Query("min_duration"))
	maxDurationStr := strings.TrimSpace(c.Query("max_duration"))

	q := h.db.WithContext(c.Request.Context()).
		Model(&models.BarbershopService{}).
		Where("barbershop_id = ?", barbershopID)

	if category != "" {
		q = q.Where("LOWER(category) = ?", category)
	}

	switch activeStr {
	case "true":
		q = q.Where("active = ?", true)
	case "false":
		q = q.Where("active = ?", false)
	}

	if query != "" {
		like := "%" + query + "%"
		q = q.Where(
			"(LOWER(name) LIKE ? OR LOWER(description) LIKE ?)",
			like,
			like,
		)
	}

	if v, err := strconv.ParseInt(minPriceStr, 10, 64); err == nil {
		q = q.Where("price >= ?", v)
	}

	if v, err := strconv.ParseInt(maxPriceStr, 10, 64); err == nil {
		q = q.Where("price <= ?", v)
	}

	if v, err := strconv.Atoi(minDurationStr); err == nil {
		q = q.Where("duration_min >= ?", v)
	}

	if v, err := strconv.Atoi(maxDurationStr); err == nil {
		q = q.Where("duration_min <= ?", v)
	}

	var services []models.BarbershopService
	if err := q.Order("id ASC").Find(&services).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_list_services"})
		return
	}

	c.JSON(http.StatusOK, services)
}

//
// ======================================================
// CREATE
// ======================================================
//

func (h *ServiceHandler) Create(c *gin.Context) {
	barbershopIDVal, ok := c.Get(middleware.ContextBarbershopID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_context"})
		return
	}
	barbershopID := barbershopIDVal.(uint)

	var req CreateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"details": err.Error(),
		})
		return
	}

	if req.Price <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_price"})
		return
	}

	svc, err := h.createUC.Execute(
		c.Request.Context(),
		serviceUC.CreateServiceInput{
			BarbershopID: barbershopID,
			Name:         req.Name,
			Description:  req.Description,
			DurationMin:  req.DurationMin,
			Price:        req.Price,
			Active:       true,
			Category:     strings.ToLower(strings.TrimSpace(req.Category)),
		},
	)
	if err != nil {
		switch err {
		case domainService.ErrInvalidContext:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_context"})
		case domainService.ErrInvalidName:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_name"})
		case domainService.ErrInvalidDuration:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_duration"})
		case domainService.ErrInvalidPrice:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_price"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_create_service"})
		}
		return
	}

	c.JSON(http.StatusCreated, svc)
}

//
// ======================================================
// UPDATE
// ======================================================
//

func (h *ServiceHandler) Update(c *gin.Context) {
	barbershopIDVal, ok := c.Get(middleware.ContextBarbershopID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_context"})
		return
	}
	barbershopID := barbershopIDVal.(uint)

	idParam := c.Param("id")
	idUint, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil || idUint == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}

	var req UpdateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"details": err.Error(),
		})
		return
	}

	if req.Price != nil && *req.Price <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_price"})
		return
	}

	if req.Category != nil {
		category := strings.ToLower(strings.TrimSpace(*req.Category))
		req.Category = &category
	}

	svc, err := h.updateUC.Execute(
		c.Request.Context(),
		serviceUC.UpdateServiceInput{
			BarbershopID: barbershopID,
			ServiceID:    uint(idUint),
			Name:         req.Name,
			Description:  req.Description,
			DurationMin:  req.DurationMin,
			Price:        req.Price,
			Active:       req.Active,
			Category:     req.Category,
		},
	)
	if err != nil {
		switch err {
		case domainService.ErrInvalidContext:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_context"})
		case domainService.ErrServiceNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "service_not_found"})
		case domainService.ErrInvalidName:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_name"})
		case domainService.ErrInvalidDuration:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_duration"})
		case domainService.ErrInvalidPrice:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_price"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_update_service"})
		}
		return
	}

	c.JSON(http.StatusOK, svc)
}
