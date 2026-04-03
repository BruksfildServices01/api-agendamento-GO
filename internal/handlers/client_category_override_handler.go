package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	domainMetrics "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
)

type ClientCategoryOverrideHandler struct {
	uc *ucMetrics.SetClientCategory
}

func NewClientCategoryOverrideHandler(
	uc *ucMetrics.SetClientCategory,
) *ClientCategoryOverrideHandler {
	return &ClientCategoryOverrideHandler{uc: uc}
}

type setClientCategoryRequest struct {
	Category     domainMetrics.ClientCategory `json:"category" binding:"required"`
	ExpiresInDays *int                        `json:"expires_in_days"` // nil = permanent
}

func (h *ClientCategoryOverrideHandler) Update(c *gin.Context) {
	clientID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	raw, exists := c.Get(middleware.ContextBarbershopID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "barbershop_context_not_found"})
		return
	}

	barbershopID := raw.(uint)

	var req setClientCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload"})
		return
	}

	var expiresAt *time.Time
	if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
		t := time.Now().UTC().AddDate(0, 0, *req.ExpiresInDays)
		expiresAt = &t
	}

	err = h.uc.Execute(
		c.Request.Context(),
		ucMetrics.SetClientCategoryInput{
			BarbershopID: barbershopID,
			ClientID:     uint(clientID),
			Category:     req.Category,
			ExpiresAt:    expiresAt,
		},
	)
	if err != nil {
		switch err.Error() {
		case "invalid_context":
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_context"})
		case "invalid_category":
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_category"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_update_category"})
		}
		return
	}

	c.Status(http.StatusNoContent)
}
