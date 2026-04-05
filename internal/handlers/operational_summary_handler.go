package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	appointmentUC "github.com/BruksfildServices01/barber-scheduler/internal/usecase/appointment"
)

type OperationalSummaryHandler struct {
	uc *appointmentUC.GetOperationalSummary
}

func NewOperationalSummaryHandler(
	uc *appointmentUC.GetOperationalSummary,
) *OperationalSummaryHandler {
	return &OperationalSummaryHandler{
		uc: uc,
	}
}

func (h *OperationalSummaryHandler) Get(c *gin.Context) {

	barbershopID := c.GetUint(middleware.ContextBarbershopID)

	if barbershopID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid_barbershop",
		})
		return
	}

	result, err := h.uc.Execute(
		c.Request.Context(),
		barbershopID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed_to_generate_summary",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}
