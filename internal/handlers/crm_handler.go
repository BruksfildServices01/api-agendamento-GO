package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/query/crm"
)

// CRMHandler serves GET /api/me/clients/:id/crm.
type CRMHandler struct {
	query *crm.Query
}

func NewCRMHandler(query *crm.Query) *CRMHandler {
	return &CRMHandler{query: query}
}

func (h *CRMHandler) Get(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)

	clientID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httperr.BadRequest(c, "bad_request", "invalid client id")
		return
	}

	resp, err := h.query.Execute(c.Request.Context(), barbershopID, uint(clientID))
	if err != nil {
		if errors.Is(err, crm.ErrClientNotFound) {
			httperr.NotFound(c, "client_not_found", "client not found")
			return
		}
		httperr.Internal(c, "internal_error", "internal server error")
		return
	}

	c.JSON(http.StatusOK, resp)
}
