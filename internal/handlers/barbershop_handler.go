package handlers

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9-]+$`)

type BarbershopHandler struct {
	db *gorm.DB
}

func NewBarbershopHandler(db *gorm.DB) *BarbershopHandler {
	return &BarbershopHandler{db: db}
}

type UpdateBarbershopConfigRequest struct {
	MinAdvanceMinutes        *int `json:"min_advance_minutes"`
	ScheduleToleranceMinutes *int `json:"schedule_tolerance_minutes"`
}

func (h *BarbershopHandler) GetMeBarbershop(c *gin.Context) {
	barbershopIDVal, _ := c.Get(middleware.ContextBarbershopID)
	barbershopID := barbershopIDVal.(uint)

	var shop models.Barbershop
	if err := h.db.First(&shop, barbershopID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
			return
		}
		httperr.Internal(c, "failed_to_get_barbershop", "Erro ao buscar dados da barbearia.")
		return
	}

	c.JSON(http.StatusOK, shop)
}

func (h *BarbershopHandler) UpdateMeBarbershop(c *gin.Context) {
	barbershopIDVal, _ := c.Get(middleware.ContextBarbershopID)
	barbershopID := barbershopIDVal.(uint)

	var shop models.Barbershop
	if err := h.db.First(&shop, barbershopID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
			return
		}
		httperr.Internal(c, "failed_to_get_barbershop", "Erro ao buscar dados da barbearia.")
		return
	}

	var req UpdateBarbershopConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "Dados inválidos na requisição.")
		return
	}

	if req.MinAdvanceMinutes != nil {
		if *req.MinAdvanceMinutes < 0 {
			httperr.BadRequest(c, "invalid_min_advance", "Antecedência mínima deve ser zero ou positiva (em minutos).")
			return
		}
		shop.MinAdvanceMinutes = *req.MinAdvanceMinutes
	}

	if req.ScheduleToleranceMinutes != nil {
		if *req.ScheduleToleranceMinutes < 0 || *req.ScheduleToleranceMinutes > 60 {
			httperr.BadRequest(c, "invalid_tolerance", "Tolerância de agenda deve estar entre 0 e 60 minutos.")
			return
		}
		shop.ScheduleToleranceMinutes = *req.ScheduleToleranceMinutes
	}

	if err := h.db.Save(&shop).Error; err != nil {
		httperr.Internal(c, "failed_to_update_barbershop", "Erro ao salvar as configurações da barbearia.")
		return
	}

	c.JSON(http.StatusOK, shop)
}

// PATCH /api/me/barbershop/slug
func (h *BarbershopHandler) UpdateSlug(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var req struct {
		Slug string `json:"slug" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "Slug inválido.")
		return
	}

	slug := strings.ToLower(strings.TrimSpace(req.Slug))

	if len(slug) < 3 || len(slug) > 50 || !slugRegex.MatchString(slug) {
		httperr.BadRequest(c, "invalid_slug", "Slug deve ter entre 3 e 50 caracteres (letras, números e hífens).")
		return
	}

	var count int64
	h.db.Model(&models.Barbershop{}).
		Where("slug = ? AND id != ?", slug, barbershopID).
		Count(&count)
	if count > 0 {
		httperr.BadRequest(c, "slug_already_exists", "Este endereço já está em uso.")
		return
	}

	if err := h.db.Model(&models.Barbershop{}).
		Where("id = ?", barbershopID).
		Update("slug", slug).Error; err != nil {
		httperr.Internal(c, "failed_to_update_slug", "Erro ao salvar.")
		return
	}

	c.JSON(http.StatusOK, gin.H{"slug": slug})
}
