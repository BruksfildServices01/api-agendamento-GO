package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
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
			httperr.BadRequest(c, "bad_request", err.Error())
		case errors.Is(err, impact.ErrBarbershopNotFound):
			httperr.NotFound(c, "barbershop_not_found", "barbershop not found")
		default:
			httperr.Internal(c, "internal_error", "internal server error")
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}
