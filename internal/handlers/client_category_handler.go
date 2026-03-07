package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
)

// Handler responsável APENAS por retornar a categoria do cliente
type ClientCategoryHandler struct {
	uc *ucMetrics.GetClientCategory
}

func NewClientCategoryHandler(
	uc *ucMetrics.GetClientCategory,
) *ClientCategoryHandler {
	return &ClientCategoryHandler{uc: uc}
}

func (h *ClientCategoryHandler) Get(c *gin.Context) {
	clientID, err := strconv.ParseUint(c.Param("id"), 10, 64)
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

	var barbershopID uint
	switch v := raw.(type) {
	case uint:
		barbershopID = v
	case int:
		barbershopID = uint(v)
	case int64:
		barbershopID = uint(v)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "invalid barbershop context type",
		})
		return
	}

	category, err := h.uc.Execute(
		c.Request.Context(),
		barbershopID,
		uint(clientID),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to load client category",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"client_id": clientID,
		"category":  string(category),
	})
}
