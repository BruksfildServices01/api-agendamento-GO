package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

type PlanHandler struct {
	createUC *subscription.CreatePlan
	listUC   *subscription.ListPlans
}

func NewPlanHandler(
	createUC *subscription.CreatePlan,
	listUC *subscription.ListPlans,
) *PlanHandler {
	return &PlanHandler{
		createUC: createUC,
		listUC:   listUC,
	}
}

type CreatePlanRequest struct {
	Name              string `json:"name" binding:"required"`
	MonthlyPriceCents int64  `json:"monthly_price_cents" binding:"min=0"`
	CutsIncluded      int    `json:"cuts_included" binding:"min=0"`
	DiscountPercent   int    `json:"discount_percent" binding:"min=0,max=100"`
}

func (h *PlanHandler) Create(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var req CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"details": err.Error(),
		})
		return
	}

	err := h.createUC.Execute(c.Request.Context(), subscription.CreatePlanInput{
		BarbershopID:      barbershopID,
		Name:              req.Name,
		MonthlyPriceCents: req.MonthlyPriceCents,
		CutsIncluded:      req.CutsIncluded,
		DiscountPercent:   req.DiscountPercent,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusCreated)
}

func (h *PlanHandler) List(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	plans, err := h.listUC.Execute(c.Request.Context(), barbershopID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_list"})
		return
	}

	c.JSON(http.StatusOK, plans)
}
