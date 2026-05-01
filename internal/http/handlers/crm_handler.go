package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/http/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/BruksfildServices01/barber-scheduler/internal/query/crm"
)

// CRMHandler serves GET /api/me/clients/:id/crm.
type CRMHandler struct {
	query *crm.Query
	audit *audit.Dispatcher
}

func NewCRMHandler(query *crm.Query, auditDispatcher *audit.Dispatcher) *CRMHandler {
	return &CRMHandler{query: query, audit: auditDispatcher}
}

func (h *CRMHandler) Get(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)

	clientID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httperr.BadRequest(c, "bad_request", "invalid client id")
		return
	}

	// Verificar se cliente está anonimizado antes de executar a query pesada do CRM
	var client models.Client
	if err := h.query.DB().
		Select("id, anonymized_at").
		Where("id = ? AND barbershop_id = ?", uint(clientID), barbershopID).
		First(&client).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.NotFound(c, "client_not_found", "client not found")
			return
		}
		httperr.Internal(c, "internal_error", "internal server error")
		return
	}
	if client.AnonymizedAt != nil {
		c.JSON(http.StatusGone, gin.H{
			"error_code": "client_anonymized",
			"message":    "Os dados deste cliente foram removidos a pedido do titular.",
		})
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

	// Auditoria de acesso a dados sensíveis/comportamentais do cliente (LGPD).
	// Apenas metadados de acesso — sem dados pessoais no payload.
	if h.audit != nil {
		cid := uint(clientID)
		userID := c.MustGet(middleware.ContextUserID).(uint)
		h.audit.Dispatch(audit.Event{
			BarbershopID: barbershopID,
			UserID:       &userID,
			Action:       "client_crm_accessed",
			Entity:       "client",
			EntityID:     &cid,
		})
	}
}
