package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/query/financial"
)

// FinancialHandler serves GET /api/me/financial.
type FinancialHandler struct {
	query *financial.Query
}

func NewFinancialHandler(query *financial.Query) *FinancialHandler {
	return &FinancialHandler{query: query}
}

// Get handles GET /api/me/financial?period=week|month
func (h *FinancialHandler) Get(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	period := financial.PeriodType(strings.TrimSpace(c.Query("period")))

	resp, err := h.query.Execute(c.Request.Context(), financial.Input{
		BarbershopID: barbershopID,
		Period:       period,
	})
	if err != nil {
		switch {
		case errors.Is(err, financial.ErrInvalidPeriod):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, financial.ErrBarbershopNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "barbershop not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}
