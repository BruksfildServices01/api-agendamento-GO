package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/usecase/paymentconfig"
)

type PaymentPolicyHandler struct {
	getUC    *paymentconfig.GetPaymentPolicies
	updateUC *paymentconfig.UpdatePaymentPolicies
}

func NewPaymentPolicyHandler(
	getUC *paymentconfig.GetPaymentPolicies,
	updateUC *paymentconfig.UpdatePaymentPolicies,
) *PaymentPolicyHandler {
	return &PaymentPolicyHandler{
		getUC:    getUC,
		updateUC: updateUC,
	}
}

func (h *PaymentPolicyHandler) Get(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	out, err := h.getUC.Execute(c.Request.Context(), barbershopID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, out)
}

func (h *PaymentPolicyHandler) Update(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var in paymentconfig.UpdatePaymentPoliciesInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload"})
		return
	}

	if err := h.updateUC.Execute(c.Request.Context(), barbershopID, in); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
