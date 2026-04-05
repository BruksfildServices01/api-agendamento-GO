package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

type SubscriptionHandler struct {
	activateUC *subscription.ActivateSubscription
	cancelUC   *subscription.CancelSubscription
	getUC      *subscription.GetActiveSubscription
	audit      *audit.Dispatcher
}

func NewSubscriptionHandler(
	activateUC *subscription.ActivateSubscription,
	cancelUC *subscription.CancelSubscription,
	getUC *subscription.GetActiveSubscription,
	auditDispatcher *audit.Dispatcher,
) *SubscriptionHandler {
	return &SubscriptionHandler{
		activateUC: activateUC,
		cancelUC:   cancelUC,
		getUC:      getUC,
		audit:      auditDispatcher,
	}
}

type ActivateSubscriptionRequest struct {
	ClientID uint `json:"client_id" binding:"required"`
	PlanID   uint `json:"plan_id" binding:"required"`
}

func (h *SubscriptionHandler) Activate(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		httperr.Unauthorized(c, "invalid_barbershop", "invalid_barbershop")
		return
	}

	var req ActivateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "invalid_request")
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
			httperr.BadRequest(c, "invalid_input", "invalid_input")

		case errors.Is(err, subscription.ErrActivateSubscriptionPlanNotFound):
			httperr.NotFound(c, "plan_not_found", "plan_not_found")

		case errors.Is(err, subscription.ErrActivateSubscriptionPlanInactive):
			httperr.BadRequest(c, "plan_inactive", "plan_inactive")

		case errors.Is(err, subscription.ErrActivateSubscriptionInvalidPlanDuration):
			httperr.BadRequest(c, "invalid_plan_duration", "invalid_plan_duration")

		case errors.Is(err, subscription.ErrActivateSubscriptionClientAlreadyHasActiveSub):
			httperr.Write(c, http.StatusConflict, "client_already_has_active_subscription", "client_already_has_active_subscription")

		default:
			httperr.Internal(c, "failed_to_activate_subscription", "failed_to_activate_subscription")
		}
		return
	}

	h.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		Action:       "subscription_activated",
		Entity:       "client",
		EntityID:     &req.ClientID,
		Metadata: map[string]any{
			"plan_id": req.PlanID,
		},
	})

	c.Status(http.StatusCreated)
}

func (h *SubscriptionHandler) Cancel(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		httperr.Unauthorized(c, "invalid_barbershop", "invalid_barbershop")
		return
	}

	clientIDParam := c.Param("clientID")
	clientID64, err := strconv.ParseUint(clientIDParam, 10, 64)
	if err != nil {
		httperr.BadRequest(c, "invalid_client_id", "invalid_client_id")
		return
	}

	clientID := uint(clientID64)
	err = h.cancelUC.Execute(c.Request.Context(), barbershopID, clientID)
	if err != nil {
		switch {
		case errors.Is(err, subscription.ErrInvalidInput):
			httperr.BadRequest(c, "invalid_input", "invalid_input")

		case errors.Is(err, subscription.ErrActiveSubscriptionNotFound):
			httperr.NotFound(c, "active_subscription_not_found", "active_subscription_not_found")

		default:
			httperr.Internal(c, "failed_to_cancel_subscription", "failed_to_cancel_subscription")
		}
		return
	}

	h.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		Action:       "subscription_cancelled",
		Entity:       "client",
		EntityID:     &clientID,
	})

	c.Status(http.StatusNoContent)
}

func (h *SubscriptionHandler) GetActive(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		httperr.Unauthorized(c, "invalid_barbershop", "invalid_barbershop")
		return
	}

	clientIDParam := c.Param("clientID")
	clientID64, err := strconv.ParseUint(clientIDParam, 10, 64)
	if err != nil {
		httperr.BadRequest(c, "invalid_client_id", "invalid_client_id")
		return
	}

	sub, err := h.getUC.Execute(c.Request.Context(), barbershopID, uint(clientID64))
	if err != nil {
		httperr.Internal(c, "failed_to_fetch", "failed_to_fetch")
		return
	}

	if sub == nil {
		c.Status(http.StatusNoContent)
		return
	}

	c.JSON(http.StatusOK, sub)
}
