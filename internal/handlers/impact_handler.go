package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/query/impact"
)

// ImpactHandler serves GET /api/me/impact.
type ImpactHandler struct {
	query *impact.Query
}

func NewImpactHandler(q *impact.Query) *ImpactHandler {
	return &ImpactHandler{query: q}
}

// Get handles GET /api/me/impact?period=week|month
func (h *ImpactHandler) Get(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	period := impact.PeriodType(strings.TrimSpace(c.Query("period")))

	resp, err := h.query.Execute(c.Request.Context(), impact.Input{
		BarbershopID: barbershopID,
		Period:       period,
	})
	if err != nil {
		switch {
		case errors.Is(err, impact.ErrInvalidPeriod):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, impact.ErrBarbershopNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "barbershop not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}
