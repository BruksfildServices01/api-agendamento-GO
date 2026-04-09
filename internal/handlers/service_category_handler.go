package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type ServiceCategoryHandler struct {
	db *gorm.DB
}

func NewServiceCategoryHandler(db *gorm.DB) *ServiceCategoryHandler {
	return &ServiceCategoryHandler{db: db}
}

type createCategoryRequest struct {
	Name string `json:"name" binding:"required"`
}

type updateCategoryRequest struct {
	Name string `json:"name" binding:"required"`
}

// GET /me/service-categories
func (h *ServiceCategoryHandler) List(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var cats []models.ServiceCategory
	if err := h.db.WithContext(c.Request.Context()).
		Where("barbershop_id = ?", barbershopID).
		Order("name ASC").
		Find(&cats).Error; err != nil {
		httperr.Internal(c, "failed_to_list_categories", "failed_to_list_categories")
		return
	}

	c.JSON(http.StatusOK, cats)
}

// POST /me/service-categories
func (h *ServiceCategoryHandler) Create(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var req createCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", err.Error())
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		httperr.BadRequest(c, "name_required", "name_required")
		return
	}

	cat := models.ServiceCategory{
		BarbershopID: barbershopID,
		Name:         name,
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&cat).Error; err != nil {
		httperr.Internal(c, "failed_to_create_category", "failed_to_create_category")
		return
	}

	c.JSON(http.StatusCreated, cat)
}

// PUT /me/service-categories/:id
func (h *ServiceCategoryHandler) Update(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		httperr.BadRequest(c, "invalid_id", "invalid_id")
		return
	}

	var req updateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", err.Error())
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		httperr.BadRequest(c, "name_required", "name_required")
		return
	}

	result := h.db.WithContext(c.Request.Context()).
		Model(&models.ServiceCategory{}).
		Where("id = ? AND barbershop_id = ?", id, barbershopID).
		Update("name", name)

	if result.Error != nil {
		httperr.Internal(c, "failed_to_update_category", "failed_to_update_category")
		return
	}
	if result.RowsAffected == 0 {
		httperr.NotFound(c, "category_not_found", "category_not_found")
		return
	}

	var cat models.ServiceCategory
	h.db.WithContext(c.Request.Context()).First(&cat, id)
	c.JSON(http.StatusOK, cat)
}

// DELETE /me/service-categories/:id
func (h *ServiceCategoryHandler) Delete(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		httperr.BadRequest(c, "invalid_id", "invalid_id")
		return
	}

	// Desvincula serviços antes de deletar
	if err := h.db.WithContext(c.Request.Context()).
		Model(&models.BarbershopService{}).
		Where("barbershop_id = ? AND category_id = ?", barbershopID, id).
		Update("category_id", nil).Error; err != nil {
		httperr.Internal(c, "failed_to_unlink_services", "failed_to_unlink_services")
		return
	}

	result := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND barbershop_id = ?", id, barbershopID).
		Delete(&models.ServiceCategory{})

	if result.Error != nil {
		httperr.Internal(c, "failed_to_delete_category", "failed_to_delete_category")
		return
	}
	if result.RowsAffected == 0 {
		httperr.NotFound(c, "category_not_found", "category_not_found")
		return
	}

	c.Status(http.StatusNoContent)
}
