package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

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
	barbershopID := c.GetUint("barbershopID")

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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusCreated)
}

func (h *SubscriptionHandler) Cancel(c *gin.Context) {
	barbershopID := c.GetUint("barbershopID")

	clientIDParam := c.Param("clientID")
	clientID64, err := strconv.ParseUint(clientIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	err = h.cancelUC.Execute(c.Request.Context(), barbershopID, uint(clientID64))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *SubscriptionHandler) GetActive(c *gin.Context) {
	barbershopID := c.GetUint("barbershopID")

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
