package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

type SubscriptionHandler struct {
	activateUC *subscription.ActivateSubscription
	cancelUC   *subscription.CancelSubscription
	getUC      *subscription.GetActiveSubscription
}

func NewSubscriptionHandler(
	activateUC *subscription.ActivateSubscription,
	cancelUC *subscription.CancelSubscription,
	getUC *subscription.GetActiveSubscription,
) *SubscriptionHandler {
	return &SubscriptionHandler{
		activateUC: activateUC,
		cancelUC:   cancelUC,
		getUC:      getUC,
	}
}

type ActivateSubscriptionRequest struct {
	ClientID uint `json:"client_id" binding:"required"`
	PlanID   uint `json:"plan_id" binding:"required"`
}

func (h *SubscriptionHandler) Activate(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_barbershop"})
		return
	}

	var req ActivateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}

	err := h.activateUC.Execute(c.Request.Context(), subscription.ActivateSubscriptionInput{
		BarbershopID: barbershopID,
		ClientID:     req.ClientID,
		PlanID:       req.PlanID,
	})
	if err != nil {
		switch {
		case errors.Is(err, subscription.ErrActivateSubscriptionInvalidInput):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_input"})

		case errors.Is(err, subscription.ErrActivateSubscriptionPlanNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "plan_not_found"})

		case errors.Is(err, subscription.ErrActivateSubscriptionPlanInactive):
			c.JSON(http.StatusBadRequest, gin.H{"error": "plan_inactive"})

		case errors.Is(err, subscription.ErrActivateSubscriptionInvalidPlanDuration):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_plan_duration"})

		case errors.Is(err, subscription.ErrActivateSubscriptionClientAlreadyHasActiveSub):
			c.JSON(http.StatusConflict, gin.H{"error": "client_already_has_active_subscription"})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_activate_subscription"})
		}
		return
	}

	c.Status(http.StatusCreated)
}

func (h *SubscriptionHandler) Cancel(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_barbershop"})
		return
	}

	clientIDParam := c.Param("clientID")
	clientID64, err := strconv.ParseUint(clientIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	err = h.cancelUC.Execute(c.Request.Context(), barbershopID, uint(clientID64))
	if err != nil {
		switch {
		case errors.Is(err, subscription.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_input"})

		case errors.Is(err, subscription.ErrActiveSubscriptionNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "active_subscription_not_found"})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_cancel_subscription"})
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *SubscriptionHandler) GetActive(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_barbershop"})
		return
	}

	clientIDParam := c.Param("clientID")
	clientID64, err := strconv.ParseUint(clientIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	sub, err := h.getUC.Execute(c.Request.Context(), barbershopID, uint(clientID64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_fetch"})
		return
	}

	if sub == nil {
		c.Status(http.StatusNoContent)
		return
	}

	c.JSON(http.StatusOK, sub)
}
