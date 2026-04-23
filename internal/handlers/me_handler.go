package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
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
		httperr.Unauthorized(c, "user_not_in_context", "user_not_in_context")
		return
	}

	userID, ok := userIDVal.(uint)
	if !ok {
		httperr.Unauthorized(c, "invalid_user_id_type", "invalid_user_id_type")
		return
	}

	var user models.User
	if err := h.db.Preload("Barbershop").First(&user, userID).Error; err != nil {
		httperr.Internal(c, "user_not_found", "user_not_found")
		return
	}

	seenTours := user.SeenTours
	if seenTours == nil {
		seenTours = models.StringSlice{}
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":            user.ID,
			"name":          user.Name,
			"email":         user.Email,
			"phone":         user.Phone,
			"role":          user.Role,
			"barbershop_id": user.BarbershopID,
			"seen_tours":    seenTours,
		},
		"barbershop": gin.H{
			"id":                      user.Barbershop.ID,
			"name":                    user.Barbershop.Name,
			"slug":                    user.Barbershop.Slug,
			"phone":                   user.Barbershop.Phone,
			"address":                 user.Barbershop.Address,
			"subscription_status":     user.Barbershop.Status,
			"trial_ends_at":           user.Barbershop.TrialEndsAt,
			"subscription_expires_at": user.Barbershop.SubscriptionExpiresAt,
		},
	})
}

func (h *MeHandler) MarkTourSeen(c *gin.Context) {
	userIDVal, exists := c.Get(middleware.ContextUserID)
	if !exists {
		httperr.Unauthorized(c, "user_not_in_context", "user_not_in_context")
		return
	}

	userID, ok := userIDVal.(uint)
	if !ok {
		httperr.Unauthorized(c, "invalid_user_id_type", "invalid_user_id_type")
		return
	}

	screenID := c.Param("screenId")
	if len(screenID) == 0 || len(screenID) > 100 {
		httperr.BadRequest(c, "invalid_screen_id", "invalid_screen_id")
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		httperr.Internal(c, "user_not_found", "user_not_found")
		return
	}

	for _, id := range user.SeenTours {
		if id == screenID {
			c.Status(http.StatusNoContent)
			return
		}
	}

	updated := append(user.SeenTours, screenID)
	if err := h.db.Model(&user).Update("seen_tours", updated).Error; err != nil {
		httperr.Internal(c, "update_failed", "update_failed")
		return
	}

	c.Status(http.StatusNoContent)
}
