package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	clienthistory "github.com/BruksfildServices01/barber-scheduler/internal/query/client_history"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
	ucSubscription "github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

type ClientHistoryHandler struct {
	service *clienthistory.Service
}

func NewClientHistoryHandler(
	db *gorm.DB,
	getClientCategory *ucMetrics.GetClientCategory,
	getActiveSubscription *ucSubscription.GetActiveSubscription,
) *ClientHistoryHandler {
	repo := clienthistory.NewRepository(db)
	service := clienthistory.NewService(
		repo,
		getClientCategory,
		getActiveSubscription,
	)

	return &ClientHistoryHandler{
		service: service,
	}
}

func (h *ClientHistoryHandler) Get(c *gin.Context) {
	clientID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid client id"})
		return
	}

	raw, exists := c.Get(middleware.ContextBarbershopID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "barbershop context not found",
		})
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "invalid barbershop context type",
		})
		return
	}

	result, err := h.service.GetClientHistory(
		c.Request.Context(),
		barbershopID,
		clientID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to load history",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}
