package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type PublicWebHandler struct {
	db *gorm.DB
}

func NewPublicWebHandler(db *gorm.DB) *PublicWebHandler {
	return &PublicWebHandler{db: db}
}

func (h *PublicWebHandler) ShowBookingPage(c *gin.Context) {
	slug := c.Param("slug")

	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		c.String(http.StatusNotFound, "Barbearia não encontrada.")
		return
	}

	category := strings.ToLower(strings.TrimSpace(c.Query("category")))
	minPriceStr := c.Query("min_price")
	maxPriceStr := c.Query("max_price")
	query := strings.ToLower(strings.TrimSpace(c.Query("query")))
	sort := strings.ToLower(strings.TrimSpace(c.Query("sort")))

	q := h.db.Where("barbershop_id = ? AND active = true", shop.ID)

	if category != "" {
		q = q.Where("LOWER(category) = ?", category)
	}

	if minPriceStr != "" {
		if minPrice, err := strconv.ParseFloat(minPriceStr, 64); err == nil {
			q = q.Where("price >= ?", minPrice)
		} else {
			c.String(http.StatusBadRequest, "min_price inválido.")
			return
		}
	}

	if maxPriceStr != "" {
		if maxPrice, err := strconv.ParseFloat(maxPriceStr, 64); err == nil {
			q = q.Where("price <= ?", maxPrice)
		} else {
			c.String(http.StatusBadRequest, "max_price inválido.")
			return
		}
	}

	if query != "" {
		like := "%" + query + "%"
		q = q.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", like, like)
	}
	orderClause := "id ASC"
	switch sort {
	case "price_asc":
		orderClause = "price ASC"
	case "price_desc":
		orderClause = "price DESC"
	case "duration_asc":
		orderClause = "duration_min ASC"
	case "duration_desc":
		orderClause = "duration_min DESC"
	}

	var products []models.BarberProduct
	if err := q.
		Order(orderClause).
		Find(&products).Error; err != nil {

		c.String(http.StatusInternalServerError, "Erro ao carregar serviços.")
		return
	}

	c.HTML(http.StatusOK, "base.html", gin.H{
		"Barbershop": shop,
		"Products":   products,
		"CurrentFilters": gin.H{
			"category":  category,
			"min_price": minPriceStr,
			"max_price": maxPriceStr,
			"query":     query,
			"sort":      sort,
		},
	})
}
