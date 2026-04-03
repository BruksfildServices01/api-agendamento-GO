package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/query/daypanel"
)

// DayPanelHandler serves GET /api/me/day-panel.
// Returns the full operational card list for the authenticated barbershop.
type DayPanelHandler struct {
	query *daypanel.Query
}

func NewDayPanelHandler(query *daypanel.Query) *DayPanelHandler {
	return &DayPanelHandler{query: query}
}

// Get handles GET /api/me/day-panel
//
// Query params:
//
//	date      — YYYY-MM-DD in the shop's local timezone (default: today)
//	barber_id — filter by a specific barber (default: all barbers)
func (h *DayPanelHandler) Get(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)

	date := strings.TrimSpace(c.Query("date"))

	var barberID uint
	if raw := strings.TrimSpace(c.Query("barber_id")); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "barber_id must be a positive integer"})
			return
		}
		barberID = uint(v)
	}

	resp, err := h.query.Execute(c.Request.Context(), daypanel.Input{
		BarbershopID: barbershopID,
		BarberID:     barberID,
		Date:         date,
	})
	if err != nil {
		switch {
		case errors.Is(err, daypanel.ErrInvalidDate):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, daypanel.ErrBarbershopNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "barbershop not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}
