package handlers

import (
	"net/http"
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

// --------- Requests ---------

type CreateBarberProductRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	DurationMin int     `json:"duration_min" binding:"required,min=1"`
	Price       float64 `json:"price" binding:"required"`
	Category    string  `json:"category"`
}

type UpdateBarberProductRequest struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	DurationMin *int     `json:"duration_min,omitempty"`
	Price       *float64 `json:"price,omitempty"`
	Active      *bool    `json:"active,omitempty"`
}

// --------- Handlers ---------
func (h *BarberProductHandler) List(c *gin.Context) {
	barbershopIDVal, _ := c.Get(middleware.ContextBarbershopID)
	barbershopID := barbershopIDVal.(uint)

	category := strings.ToLower(strings.TrimSpace(c.Query("category")))
	activeStr := strings.TrimSpace(c.Query("active")) // "true", "false" ou vazio
	query := strings.ToLower(strings.TrimSpace(c.Query("query")))

	q := h.db.Where("barbershop_id = ?", barbershopID)

	if category != "" {
		q = q.Where("LOWER(category) = ?", category)
	}

	if activeStr != "" {
		if activeStr == "true" {
			q = q.Where("active = ?", true)
		} else if activeStr == "false" {
			q = q.Where("active = ?", false)
		}
	}

	if query != "" {
		like := "%" + query + "%"
		q = q.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", like, like)
	}

	var products []models.BarberProduct
	if err := q.
		Order("id ASC").
		Find(&products).Error; err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_list_products"})
		return
	}

	c.JSON(http.StatusOK, products)
}

func (h *BarberProductHandler) Create(c *gin.Context) {
	barbershopIDVal, _ := c.Get(middleware.ContextBarbershopID)
	barbershopID := barbershopIDVal.(uint)

	var req CreateBarberProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"details": err.Error(),
		})
		return
	}

	product := models.BarberProduct{
		BarbershopID: barbershopID,
		Name:         req.Name,
		Description:  req.Description,
		DurationMin:  req.DurationMin,
		Price:        req.Price,
		Active:       true,
		Category:     strings.ToLower(req.Category),
	}

	if err := h.db.Create(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_create_product"})
		return
	}

	c.JSON(http.StatusCreated, product)
}

func (h *BarberProductHandler) Update(c *gin.Context) {
	barbershopIDVal, _ := c.Get(middleware.ContextBarbershopID)
	barbershopID := barbershopIDVal.(uint)

	id := c.Param("id")

	var product models.BarberProduct
	if err := h.db.
		Where("id = ? AND barbershop_id = ?", id, barbershopID).
		First(&product).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "product_not_found"})
			return
		}
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
		product.Name = *req.Name
	}
	if req.Description != nil {
		product.Description = *req.Description
	}
	if req.DurationMin != nil {
		product.DurationMin = *req.DurationMin
	}
	if req.Price != nil {
		product.Price = *req.Price
	}
	if req.Active != nil {
		product.Active = *req.Active
	}

	if err := h.db.Save(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_update_product"})
		return
	}

	c.JSON(http.StatusOK, product)
}
