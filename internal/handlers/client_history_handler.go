package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	clienthistory "github.com/BruksfildServices01/barber-scheduler/internal/query/client_history"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
	ucSubscription "github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

type ClientHistoryHandler struct {
	db      *gorm.DB
	service *clienthistory.Service
	audit   *audit.Dispatcher
}

func NewClientHistoryHandler(
	db *gorm.DB,
	getClientCategory *ucMetrics.GetClientCategory,
	getActiveSubscription *ucSubscription.GetActiveSubscription,
	auditDispatcher *audit.Dispatcher,
) *ClientHistoryHandler {
	repo := clienthistory.NewRepository(db)
	service := clienthistory.NewService(
		repo,
		getClientCategory,
		getActiveSubscription,
	)

	return &ClientHistoryHandler{
		db:      db,
		service: service,
		audit:   auditDispatcher,
	}
}

func (h *ClientHistoryHandler) Get(c *gin.Context) {
	clientID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		httperr.BadRequest(c, "bad_request", "invalid client id")
		return
	}

	raw, exists := c.Get(middleware.ContextBarbershopID)
	if !exists {
		httperr.Unauthorized(c, "barbershop_context_not_found", "barbershop context not found")
		return
	}

	var barbershopID int64
	switch v := raw.(type) {
	case uint:
		barbershopID = int64(v)
	case int:
		barbershopID = int64(v)
	case int64:
		barbershopID = v
	default:
		httperr.Internal(c, "internal_error", "invalid barbershop context type")
		return
	}

	// Verificar anonimização antes de expor histórico
	var anonymizedAt *string
	if err := h.db.WithContext(c.Request.Context()).
		Raw("SELECT anonymized_at FROM clients WHERE id = ? AND barbershop_id = ?", clientID, barbershopID).
		Scan(&anonymizedAt).Error; err == nil && anonymizedAt != nil {
		c.JSON(http.StatusGone, gin.H{
			"error_code": "client_anonymized",
			"message":    "Os dados deste cliente foram removidos a pedido do titular.",
		})
		return
	}

	result, err := h.service.GetClientHistory(
		c.Request.Context(),
		barbershopID,
		clientID,
	)
	if err != nil {
		httperr.Internal(c, "internal_error", "failed to load history")
		return
	}

	c.JSON(http.StatusOK, result)

	// Auditoria de acesso ao histórico completo do cliente (LGPD).
	// Apenas metadados de acesso — sem dados pessoais no payload.
	if h.audit != nil {
		cid := uint(clientID)
		userID := c.MustGet(middleware.ContextUserID).(uint)
		h.audit.Dispatch(audit.Event{
			BarbershopID: uint(barbershopID),
			UserID:       &userID,
			Action:       "client_history_accessed",
			Entity:       "client",
			EntityID:     &cid,
		})
	}
}
