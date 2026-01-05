package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type MeHandler struct {
	db *gorm.DB
}

func NewMeHandler(db *gorm.DB) *MeHandler {
	return &MeHandler{db: db}
}

func (h *MeHandler) GetMe(c *gin.Context) {
	userIDVal, exists := c.Get(middleware.ContextUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_not_in_context"})
		return
	}

	userID, ok := userIDVal.(uint)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_user_id_type"})
		return
	}

	var user models.User
	if err := h.db.Preload("Barbershop").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user_not_found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":            user.ID,
			"name":          user.Name,
			"email":         user.Email,
			"phone":         user.Phone,
			"role":          user.Role,
			"barbershop_id": user.BarbershopID,
		},
		"barbershop": gin.H{
			"id":      user.Barbershop.ID,
			"name":    user.Barbershop.Name,
			"slug":    user.Barbershop.Slug,
			"phone":   user.Barbershop.Phone,
			"address": user.Barbershop.Address,
		},
	})
}
