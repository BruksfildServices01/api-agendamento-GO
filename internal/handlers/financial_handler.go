package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
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
			httperr.BadRequest(c, "bad_request", err.Error())
		case errors.Is(err, financial.ErrBarbershopNotFound):
			httperr.NotFound(c, "barbershop_not_found", "barbershop not found")
		default:
			httperr.Internal(c, "internal_error", "internal server error")
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}
