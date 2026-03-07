package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type BarberProductHandler struct {
	db *gorm.DB
}

func NewBarberProductHandler(db *gorm.DB) *BarberProductHandler {
	return &BarberProductHandler{db: db}
}

//
// ======================================================
// REQUEST DTOs (JSON usa CENTAVOS)
// ======================================================
//

type CreateBarberProductRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	DurationMin int    `json:"duration_min" binding:"required,min=1"`
	Price       int64  `json:"price" binding:"required"` // cents
	Category    string `json:"category"`
}

type UpdateBarberProductRequest struct {
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

func (h *BarberProductHandler) List(c *gin.Context) {
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

	// filtros em CENTAVOS
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

	var products []models.BarbershopService
	if err := q.Order("id ASC").Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_list_products"})
		return
	}

	c.JSON(http.StatusOK, products)
}

//
// ======================================================
// CREATE
// ======================================================
//

func (h *BarberProductHandler) Create(c *gin.Context) {
	barbershopIDVal, ok := c.Get(middleware.ContextBarbershopID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_context"})
		return
	}
	barbershopID := barbershopIDVal.(uint)

	var req CreateBarberProductRequest
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

	product := models.BarbershopService{
		BarbershopID: &barbershopID,
		Name:         strings.TrimSpace(req.Name),
		Description:  strings.TrimSpace(req.Description),
		DurationMin:  req.DurationMin,
		Price:        req.Price, // cents
		Active:       true,
		Category:     strings.ToLower(strings.TrimSpace(req.Category)),
	}

	if err := h.db.WithContext(c.Request.Context()).
		Create(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_create_product"})
		return
	}

	c.JSON(http.StatusCreated, product)
}

//
// ======================================================
// UPDATE
// ======================================================
//

func (h *BarberProductHandler) Update(c *gin.Context) {
	barbershopIDVal, ok := c.Get(middleware.ContextBarbershopID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_context"})
		return
	}
	barbershopID := barbershopIDVal.(uint)

	idParam := c.Param("id")
	idUint, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}

	var product models.BarbershopService

	err = h.db.WithContext(c.Request.Context()).
		Where("id = ? AND barbershop_id = ?", uint(idUint), barbershopID).
		First(&product).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "product_not_found"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_get_product"})
		return
	}

	var req UpdateBarberProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"details": err.Error(),
		})
		return
	}

	if req.Name != nil {
		product.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		product.Description = strings.TrimSpace(*req.Description)
	}
	if req.DurationMin != nil {
		product.DurationMin = *req.DurationMin
	}
	if req.Price != nil {
		if *req.Price <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_price"})
			return
		}
		product.Price = *req.Price // cents
	}
	if req.Active != nil {
		product.Active = *req.Active
	}
	if req.Category != nil {
		product.Category = strings.ToLower(strings.TrimSpace(*req.Category))
	}

	if err := h.db.WithContext(c.Request.Context()).
		Save(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_update_product"})
		return
	}

	c.JSON(http.StatusOK, product)
}
