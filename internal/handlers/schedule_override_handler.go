package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type ScheduleOverrideHandler struct {
	db *gorm.DB
}

func NewScheduleOverrideHandler(db *gorm.DB) *ScheduleOverrideHandler {
	return &ScheduleOverrideHandler{db: db}
}

// ScheduleOverrideResponse is the JSON shape returned to the frontend.
type ScheduleOverrideResponse struct {
	ID           uint    `json:"id"`
	BarbershopID uint    `json:"barbershop_id"`
	BarberID     uint    `json:"barber_id"`
	Date         *string `json:"date"`    // "YYYY-MM-DD" or null
	Weekday      *int    `json:"weekday"` // 0-6 or null
	Month        *int    `json:"month"`   // 1-12 or null
	Year         *int    `json:"year"`    // or null
	Closed       bool    `json:"closed"`
	StartTime    string  `json:"start_time"`
	EndTime      string  `json:"end_time"`
}

func overrideToResponse(o *models.ScheduleOverride) ScheduleOverrideResponse {
	r := ScheduleOverrideResponse{
		ID:           o.ID,
		BarbershopID: o.BarbershopID,
		BarberID:     o.BarberID,
		Weekday:      o.Weekday,
		Month:        o.Month,
		Year:         o.Year,
		Closed:       o.Closed,
		StartTime:    o.StartTime,
		EndTime:      o.EndTime,
	}
	if o.Date != nil {
		s := o.Date.Format("2006-01-02")
		r.Date = &s
	}
	return r
}

// ======================================================
// GET /me/schedule-overrides?date=YYYY-MM-DD
// ======================================================

// List retorna as exceções de horário relevantes para a data informada:
// a exceção por data específica e/ou a exceção por dia-da-semana no mês.
func (h *ScheduleOverrideHandler) List(c *gin.Context) {
	barberID := c.MustGet(middleware.ContextUserID).(uint)
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	dateParam := c.Query("date") // "YYYY-MM-DD"
	if dateParam == "" {
		httperr.BadRequest(c, "missing_date", "query param 'date' is required")
		return
	}

	parsed, err := time.Parse("2006-01-02", dateParam)
	if err != nil {
		httperr.BadRequest(c, "invalid_date", "date must be YYYY-MM-DD")
		return
	}

	weekday := int(parsed.Weekday())
	month := int(parsed.Month())
	year := parsed.Year()

	var overrides []models.ScheduleOverride
	if err := h.db.
		Where(`barbershop_id = ? AND barber_id = ?
			   AND (
			     date = ?
			     OR (weekday = ? AND month = ? AND year = ?)
			   )`,
			barbershopID, barberID,
			dateParam,
			weekday, month, year,
		).
		Find(&overrides).Error; err != nil {
		httperr.Internal(c, "failed_to_list_overrides", err.Error())
		return
	}

	resp := make([]ScheduleOverrideResponse, 0, len(overrides))
	for i := range overrides {
		resp = append(resp, overrideToResponse(&overrides[i]))
	}

	c.JSON(http.StatusOK, resp)
}

// ======================================================
// PUT /me/schedule-overrides
// ======================================================

type UpsertScheduleOverrideRequest struct {
	// Escopo: preencher Date OU (Weekday + Month + Year)
	Date    *string `json:"date"`    // "YYYY-MM-DD"
	Weekday *int    `json:"weekday"` // 0-6
	Month   *int    `json:"month"`   // 1-12
	Year    *int    `json:"year"`

	Closed    bool   `json:"closed"`
	StartTime string `json:"start_time"` // HH:MM
	EndTime   string `json:"end_time"`   // HH:MM
}

func (h *ScheduleOverrideHandler) Upsert(c *gin.Context) {
	barberID := c.MustGet(middleware.ContextUserID).(uint)
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var req UpsertScheduleOverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", err.Error())
		return
	}

	// Validação de escopo
	hasDate := req.Date != nil && *req.Date != ""
	hasWeekday := req.Weekday != nil && req.Month != nil && req.Year != nil

	if !hasDate && !hasWeekday {
		httperr.BadRequest(c, "invalid_scope", "provide either 'date' or 'weekday'+'month'+'year'")
		return
	}
	if hasDate && hasWeekday {
		httperr.BadRequest(c, "invalid_scope", "provide 'date' OR 'weekday'+'month'+'year', not both")
		return
	}

	// Validação de comportamento
	if !req.Closed && (req.StartTime == "" || req.EndTime == "") {
		httperr.BadRequest(c, "invalid_hours", "when not closed, start_time and end_time are required")
		return
	}

	var existing models.ScheduleOverride
	var query *gorm.DB

	if hasDate {
		parsedDate, err := time.Parse("2006-01-02", *req.Date)
		if err != nil {
			httperr.BadRequest(c, "invalid_date", "date must be YYYY-MM-DD")
			return
		}
		query = h.db.Where("barbershop_id = ? AND barber_id = ? AND date = ?",
			barbershopID, barberID, parsedDate.Format("2006-01-02"))
	} else {
		query = h.db.Where("barbershop_id = ? AND barber_id = ? AND weekday = ? AND month = ? AND year = ?",
			barbershopID, barberID, *req.Weekday, *req.Month, *req.Year)
	}

	result := query.First(&existing)

	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		httperr.Internal(c, "db_error", result.Error.Error())
		return
	}

	isNew := errors.Is(result.Error, gorm.ErrRecordNotFound)

	if isNew {
		override := models.ScheduleOverride{
			BarbershopID: barbershopID,
			BarberID:     barberID,
			Weekday:      req.Weekday,
			Month:        req.Month,
			Year:         req.Year,
			Closed:       req.Closed,
			StartTime:    req.StartTime,
			EndTime:      req.EndTime,
		}
		if hasDate {
			parsedDate, _ := time.Parse("2006-01-02", *req.Date)
			override.Date = &parsedDate
		}
		if err := h.db.Create(&override).Error; err != nil {
			httperr.Internal(c, "failed_to_create_override", err.Error())
			return
		}
		c.JSON(http.StatusCreated, overrideToResponse(&override))
		return
	}

	// Update existing
	existing.Closed = req.Closed
	existing.StartTime = req.StartTime
	existing.EndTime = req.EndTime
	if err := h.db.Save(&existing).Error; err != nil {
		httperr.Internal(c, "failed_to_update_override", err.Error())
		return
	}

	c.JSON(http.StatusOK, overrideToResponse(&existing))
}

// ======================================================
// DELETE /me/schedule-overrides/:id
// ======================================================

func (h *ScheduleOverrideHandler) Delete(c *gin.Context) {
	barberID := c.MustGet(middleware.ContextUserID).(uint)
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		httperr.BadRequest(c, "invalid_id", "id must be a number")
		return
	}

	result := h.db.
		Where("id = ? AND barbershop_id = ? AND barber_id = ?", id, barbershopID, barberID).
		Delete(&models.ScheduleOverride{})

	if result.Error != nil {
		httperr.Internal(c, "failed_to_delete_override", result.Error.Error())
		return
	}
	if result.RowsAffected == 0 {
		httperr.NotFound(c, "override_not_found", "override not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
