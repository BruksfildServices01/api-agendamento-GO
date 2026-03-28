package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	domainProduct "github.com/BruksfildServices01/barber-scheduler/internal/domain/product"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	productUC "github.com/BruksfildServices01/barber-scheduler/internal/usecase/product"
)

type ProductHandler struct {
	db       *gorm.DB
	createUC *productUC.CreateProduct
	updateUC *productUC.UpdateProduct
}

func NewProductHandler(
	db *gorm.DB,
	createUC *productUC.CreateProduct,
	updateUC *productUC.UpdateProduct,
) *ProductHandler {
	return &ProductHandler{
		db:       db,
		createUC: createUC,
		updateUC: updateUC,
	}
}

//
// ======================================================
// REQUEST DTOs
// ======================================================
//

type CreateProductRequest struct {
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	Category      string `json:"category"`
	Price         int64  `json:"price" binding:"required"`
	Stock         int    `json:"stock"`
	Active        bool   `json:"active"`
	OnlineVisible bool   `json:"online_visible"`
}

type UpdateProductRequest struct {
	Name          *string `json:"name,omitempty"`
	Description   *string `json:"description,omitempty"`
	Category      *string `json:"category,omitempty"`
	Price         *int64  `json:"price,omitempty"`
	Stock         *int    `json:"stock,omitempty"`
	Active        *bool   `json:"active,omitempty"`
	OnlineVisible *bool   `json:"online_visible,omitempty"`
}

//
// ======================================================
// LIST
// ======================================================
//
// Conservador: mantém listagem via GORM para preservar filtros.
// Depois migramos para use case/query dedicada se quiser.
//

func (h *ProductHandler) List(c *gin.Context) {
	barbershopIDVal, ok := c.Get(middleware.ContextBarbershopID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_context"})
		return
	}
	barbershopID := barbershopIDVal.(uint)

	category := strings.ToLower(strings.TrimSpace(c.Query("category")))
	activeStr := strings.TrimSpace(c.Query("active"))
	onlineVisibleStr := strings.TrimSpace(c.Query("online_visible"))
	query := strings.ToLower(strings.TrimSpace(c.Query("query")))
	minPriceStr := strings.TrimSpace(c.Query("min_price"))
	maxPriceStr := strings.TrimSpace(c.Query("max_price"))
	minStockStr := strings.TrimSpace(c.Query("min_stock"))
	maxStockStr := strings.TrimSpace(c.Query("max_stock"))

	q := h.db.WithContext(c.Request.Context()).
		Model(&models.Product{}).
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

	switch onlineVisibleStr {
	case "true":
		q = q.Where("online_visible = ?", true)
	case "false":
		q = q.Where("online_visible = ?", false)
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

	if v, err := strconv.Atoi(minStockStr); err == nil {
		q = q.Where("stock >= ?", v)
	}

	if v, err := strconv.Atoi(maxStockStr); err == nil {
		q = q.Where("stock <= ?", v)
	}

	var products []models.Product
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

func (h *ProductHandler) Create(c *gin.Context) {
	barbershopIDVal, ok := c.Get(middleware.ContextBarbershopID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_context"})
		return
	}
	barbershopID := barbershopIDVal.(uint)

	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"details": err.Error(),
		})
		return
	}

	product, err := h.createUC.Execute(
		c.Request.Context(),
		productUC.CreateProductInput{
			BarbershopID:  barbershopID,
			Name:          req.Name,
			Description:   req.Description,
			Category:      strings.ToLower(strings.TrimSpace(req.Category)),
			Price:         req.Price,
			Stock:         req.Stock,
			Active:        req.Active,
			OnlineVisible: req.OnlineVisible,
		},
	)
	if err != nil {
		switch err.Error() {
		case "invalid_barbershop_id":
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_context"})
		case "invalid_name":
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_name"})
		case "invalid_price":
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_price"})
		case "invalid_stock":
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_stock"})
		case "invalid_online_visible_without_stock":
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_online_visible_without_stock"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_create_product"})
		}
		return
	}

	c.JSON(http.StatusCreated, product)
}

//
// ======================================================
// UPDATE
// ======================================================
//

func (h *ProductHandler) Update(c *gin.Context) {
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

	var req UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"details": err.Error(),
		})
		return
	}

	if req.Category != nil {
		category := strings.ToLower(strings.TrimSpace(*req.Category))
		req.Category = &category
	}

	product, err := h.updateUC.Execute(
		c.Request.Context(),
		productUC.UpdateProductInput{
			BarbershopID:  barbershopID,
			ProductID:     uint(idUint),
			Name:          req.Name,
			Description:   req.Description,
			Category:      req.Category,
			Price:         req.Price,
			Stock:         req.Stock,
			Active:        req.Active,
			OnlineVisible: req.OnlineVisible,
		},
	)
	if err != nil {
		switch err.Error() {
		case "invalid_context":
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_context"})
		case "invalid_name":
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_name"})
		case "invalid_price":
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_price"})
		case "invalid_stock":
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_stock"})
		case "invalid_online_visible_without_stock":
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_online_visible_without_stock"})
		default:
			if err == domainProduct.ErrProductNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "product_not_found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_update_product"})
		}
		return
	}

	c.JSON(http.StatusOK, product)
}
