package handlers

import (
	"errors"
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
	DurationDays      int    `json:"duration_days" binding:"required"`
	CutsIncluded      int    `json:"cuts_included" binding:"min=0"`
	DiscountPercent   int    `json:"discount_percent" binding:"min=0,max=100"`
	ServiceIDs        []uint `json:"service_ids" binding:"required"`
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
		DurationDays:      req.DurationDays,
		CutsIncluded:      req.CutsIncluded,
		DiscountPercent:   req.DiscountPercent,
		ServiceIDs:        req.ServiceIDs,
	})
	if err != nil {
		switch {
		case errors.Is(err, subscription.ErrInvalidBarbershop):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_barbershop"})

		case errors.Is(err, subscription.ErrInvalidName):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_name"})

		case errors.Is(err, subscription.ErrInvalidPrice):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_price"})

		case errors.Is(err, subscription.ErrInvalidPlanDuration):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_duration_days"})

		case errors.Is(err, subscription.ErrInvalidCutsIncluded):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_cuts_included"})

		case errors.Is(err, subscription.ErrInvalidDiscount):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_discount"})

		case errors.Is(err, subscription.ErrServiceIDsRequired):
			c.JSON(http.StatusBadRequest, gin.H{"error": "service_ids_required"})

		case errors.Is(err, subscription.ErrInvalidServiceID):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_service_id"})

		case errors.Is(err, subscription.ErrInvalidServiceIDs):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_service_ids"})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_create_plan"})
		}
		return
	}

	c.Status(http.StatusCreated)
}

func (h *PlanHandler) List(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	plans, err := h.listUC.Execute(c.Request.Context(), barbershopID)
	if err != nil {
		switch {
		case errors.Is(err, subscription.ErrInvalidBarbershop):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_barbershop"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_list"})
		}
		return
	}

	c.JSON(http.StatusOK, plans)
}
