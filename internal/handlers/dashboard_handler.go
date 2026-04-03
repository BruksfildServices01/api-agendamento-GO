package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/query/dashboard"
)

// DashboardHandler serves GET /api/me/dashboard.
type DashboardHandler struct {
	query *dashboard.Query
}

func NewDashboardHandler(query *dashboard.Query) *DashboardHandler {
	return &DashboardHandler{query: query}
}

// Get handles GET /api/me/dashboard?period=day|week|month
func (h *DashboardHandler) Get(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	period := dashboard.PeriodType(strings.TrimSpace(c.Query("period")))

	resp, err := h.query.Execute(c.Request.Context(), dashboard.Input{
		BarbershopID: barbershopID,
		Period:       period,
	})
	if err != nil {
		switch {
		case errors.Is(err, dashboard.ErrInvalidPeriod):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, dashboard.ErrBarbershopNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "barbershop not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}
