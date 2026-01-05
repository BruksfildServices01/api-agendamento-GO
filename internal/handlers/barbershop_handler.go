package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type BarbershopHandler struct {
	db *gorm.DB
}

func NewBarbershopHandler(db *gorm.DB) *BarbershopHandler {
	return &BarbershopHandler{db: db}
}

type UpdateBarbershopConfigRequest struct {
	MinAdvanceMinutes *int `json:"min_advance_minutes"`
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

	if err := h.db.Save(&shop).Error; err != nil {
		httperr.Internal(c, "failed_to_update_barbershop", "Erro ao salvar as configurações da barbearia.")
		return
	}

	c.JSON(http.StatusOK, shop)
}
