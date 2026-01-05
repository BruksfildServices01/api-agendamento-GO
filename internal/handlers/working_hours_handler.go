package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type WorkingHoursHandler struct {
	db *gorm.DB
}

func NewWorkingHoursHandler(db *gorm.DB) *WorkingHoursHandler {
	return &WorkingHoursHandler{db: db}
}

type WorkingDayConfig struct {
	Weekday    int    `json:"weekday" binding:"required,min=0,max=6"`
	Active     bool   `json:"active"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	LunchStart string `json:"lunch_start"`
	LunchEnd   string `json:"lunch_end"`
}

type WorkingHoursUpdateRequest struct {
	Days []WorkingDayConfig `json:"days" binding:"required"`
}

func (h *WorkingHoursHandler) Get(c *gin.Context) {
	userIDVal, _ := c.Get(middleware.ContextUserID)
	barberID := userIDVal.(uint)

	var hours []models.WorkingHours
	if err := h.db.
		Where("barber_id = ?", barberID).
		Order("weekday ASC").
		Find(&hours).Error; err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_get_working_hours"})
		return
	}

	c.JSON(http.StatusOK, hours)
}

func (h *WorkingHoursHandler) Update(c *gin.Context) {
	userIDVal, _ := c.Get(middleware.ContextUserID)
	barberID := userIDVal.(uint)

	var req WorkingHoursUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"details": err.Error(),
		})
		return
	}

	if err := h.db.Where("barber_id = ?", barberID).Delete(&models.WorkingHours{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_clear_existing_hours"})
		return
	}

	var toCreate []models.WorkingHours
	for _, d := range req.Days {
		wh := models.WorkingHours{
			BarberID:   barberID,
			Weekday:    d.Weekday,
			Active:     d.Active,
			StartTime:  d.StartTime,
			EndTime:    d.EndTime,
			LunchStart: d.LunchStart,
			LunchEnd:   d.LunchEnd,
		}
		toCreate = append(toCreate, wh)
	}

	if len(toCreate) > 0 {
		if err := h.db.Create(&toCreate).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_save_working_hours"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
