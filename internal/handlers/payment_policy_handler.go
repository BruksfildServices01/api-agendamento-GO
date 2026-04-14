package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/usecase/paymentconfig"
)

type PaymentPolicyHandler struct {
	getUC    *paymentconfig.GetPaymentPolicies
	updateUC *paymentconfig.UpdatePaymentPolicies
	audit    *audit.Dispatcher
}

func NewPaymentPolicyHandler(
	getUC *paymentconfig.GetPaymentPolicies,
	updateUC *paymentconfig.UpdatePaymentPolicies,
	auditDispatcher *audit.Dispatcher,
) *PaymentPolicyHandler {
	return &PaymentPolicyHandler{
		getUC:    getUC,
		updateUC: updateUC,
		audit:    auditDispatcher,
	}
}

func (h *PaymentPolicyHandler) Get(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	out, err := h.getUC.Execute(c.Request.Context(), barbershopID)
	if err != nil {
		httperr.Internal(c, "internal_error", err.Error())
		return
	}

	c.JSON(http.StatusOK, out)
}

func (h *PaymentPolicyHandler) Update(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var in paymentconfig.UpdatePaymentPoliciesInput
	if err := c.ShouldBindJSON(&in); err != nil {
		httperr.BadRequest(c, "invalid_payload", "invalid_payload")
		return
	}

	if err := h.updateUC.Execute(c.Request.Context(), barbershopID, in); err != nil {
		if err == paymentconfig.ErrInvalidMPCredentials {
			httperr.BadRequest(c, "invalid_mp_credentials", "Token do Mercado Pago inválido. Verifique suas credenciais.")
			return
		}
		httperr.Internal(c, "internal_error", err.Error())
		return
	}

	userID := c.MustGet(middleware.ContextUserID).(uint)
	h.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		UserID:       &userID,
		Action:       "payment_policy_updated",
		Entity:       "barbershop",
		EntityID:     &barbershopID,
	})

	c.Status(http.StatusNoContent)
}
