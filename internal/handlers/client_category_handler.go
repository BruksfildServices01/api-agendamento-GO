package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
	ucSubscription "github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

type ClientCategoryHandler struct {
	getCategoryUC     *ucMetrics.GetClientCategory
	getSubscriptionUC *ucSubscription.GetActiveSubscription
}

func NewClientCategoryHandler(
	getCategoryUC *ucMetrics.GetClientCategory,
	getSubscriptionUC *ucSubscription.GetActiveSubscription,
) *ClientCategoryHandler {
	return &ClientCategoryHandler{
		getCategoryUC:     getCategoryUC,
		getSubscriptionUC: getSubscriptionUC,
	}
}

func (h *ClientCategoryHandler) Get(c *gin.Context) {
	clientID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httperr.BadRequest(c, "invalid_client_id", "invalid_client_id")
		return
	}

	raw, exists := c.Get(middleware.ContextBarbershopID)
	if !exists {
		httperr.Unauthorized(c, "barbershop_context_not_found", "barbershop_context_not_found")
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
		httperr.Internal(c, "invalid_barbershop_context_type", "invalid_barbershop_context_type")
		return
	}

	category, err := h.getCategoryUC.Execute(
		c.Request.Context(),
		barbershopID,
		uint(clientID),
	)
	if err != nil {
		httperr.Internal(c, "failed_to_load_client_category", "failed_to_load_client_category")
		return
	}

	premium := false

	sub, err := h.getSubscriptionUC.Execute(
		c.Request.Context(),
		barbershopID,
		uint(clientID),
	)
	if err != nil {
		httperr.Internal(c, "failed_to_load_client_subscription", "failed_to_load_client_subscription")
		return
	}

	if sub != nil {
		premium = true
	}

	c.JSON(http.StatusOK, gin.H{
		"client_id": clientID,
		"category":  string(category),
		"premium":   premium,
	})
}
