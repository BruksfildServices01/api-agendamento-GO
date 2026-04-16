package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

type PlanHandler struct {
	createUC      *subscription.CreatePlan
	updateUC      *subscription.UpdatePlan
	setActiveUC   *subscription.SetPlanActive
	listUC        *subscription.ListPlans
	deleteUC      *subscription.DeletePlan
}

func NewPlanHandler(
	createUC *subscription.CreatePlan,
	updateUC *subscription.UpdatePlan,
	setActiveUC *subscription.SetPlanActive,
	listUC *subscription.ListPlans,
	deleteUC *subscription.DeletePlan,
) *PlanHandler {
	return &PlanHandler{
		createUC:    createUC,
		updateUC:    updateUC,
		setActiveUC: setActiveUC,
		listUC:      listUC,
		deleteUC:    deleteUC,
	}
}

type CreatePlanRequest struct {
	Name              string `json:"name" binding:"required"`
	MonthlyPriceCents int64  `json:"monthly_price_cents" binding:"min=0"`
	DurationDays      int    `json:"duration_days" binding:"required"`
	CutsIncluded      int    `json:"cuts_included" binding:"min=0"`
	DiscountPercent   int    `json:"discount_percent" binding:"min=0,max=100"`
	ServiceIDs        []uint `json:"service_ids"`
	CategoryIDs       []uint `json:"category_ids"`
}

func (h *PlanHandler) Create(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var req CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", err.Error())
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
		CategoryIDs:       req.CategoryIDs,
	})
	if err != nil {
		switch {
		case errors.Is(err, subscription.ErrInvalidBarbershop):
			httperr.BadRequest(c, "invalid_barbershop", "invalid_barbershop")

		case errors.Is(err, subscription.ErrInvalidName):
			httperr.BadRequest(c, "invalid_name", "invalid_name")

		case errors.Is(err, subscription.ErrInvalidPrice):
			httperr.BadRequest(c, "invalid_price", "invalid_price")

		case errors.Is(err, subscription.ErrInvalidPlanDuration):
			httperr.BadRequest(c, "invalid_duration_days", "invalid_duration_days")

		case errors.Is(err, subscription.ErrInvalidCutsIncluded):
			httperr.BadRequest(c, "invalid_cuts_included", "invalid_cuts_included")

		case errors.Is(err, subscription.ErrInvalidDiscount):
			httperr.BadRequest(c, "invalid_discount", "invalid_discount")

		case errors.Is(err, subscription.ErrServiceIDsRequired):
			httperr.BadRequest(c, "service_ids_required", "service_ids_required")

		case errors.Is(err, subscription.ErrInvalidServiceID):
			httperr.BadRequest(c, "invalid_service_id", "invalid_service_id")

		case errors.Is(err, subscription.ErrInvalidServiceIDs):
			httperr.BadRequest(c, "invalid_service_ids", "invalid_service_ids")

		default:
			httperr.Internal(c, "failed_to_create_plan", "failed_to_create_plan")
		}
		return
	}

	c.Status(http.StatusCreated)
}

func (h *PlanHandler) Update(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	planIDParam := c.Param("id")
	planID64, err := strconv.ParseUint(planIDParam, 10, 64)
	if err != nil {
		httperr.BadRequest(c, "invalid_plan_id", "invalid_plan_id")
		return
	}

	var req CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", err.Error())
		return
	}

	if err := h.updateUC.Execute(c.Request.Context(), subscription.UpdatePlanInput{
		BarbershopID:      barbershopID,
		PlanID:            uint(planID64),
		Name:              req.Name,
		MonthlyPriceCents: req.MonthlyPriceCents,
		DurationDays:      req.DurationDays,
		CutsIncluded:      req.CutsIncluded,
		DiscountPercent:   req.DiscountPercent,
		ServiceIDs:        req.ServiceIDs,
		CategoryIDs:       req.CategoryIDs,
	}); err != nil {
		switch {
		case errors.Is(err, subscription.ErrInvalidInput):
			httperr.BadRequest(c, "invalid_input", "invalid_input")
		case errors.Is(err, subscription.ErrInvalidName):
			httperr.BadRequest(c, "invalid_name", "invalid_name")
		case errors.Is(err, subscription.ErrInvalidPrice):
			httperr.BadRequest(c, "invalid_price", "invalid_price")
		case errors.Is(err, subscription.ErrInvalidPlanDuration):
			httperr.BadRequest(c, "invalid_duration_days", "invalid_duration_days")
		case errors.Is(err, subscription.ErrInvalidCutsIncluded):
			httperr.BadRequest(c, "invalid_cuts_included", "invalid_cuts_included")
		case errors.Is(err, subscription.ErrInvalidDiscount):
			httperr.BadRequest(c, "invalid_discount", "invalid_discount")
		case errors.Is(err, subscription.ErrServiceIDsRequired):
			httperr.BadRequest(c, "service_ids_required", "service_ids_required")
		case errors.Is(err, subscription.ErrInvalidServiceIDs):
			httperr.BadRequest(c, "invalid_service_ids", "invalid_service_ids")
		case errors.Is(err, subscription.ErrPlanUpdateNotFound):
			httperr.NotFound(c, "plan_not_found", "plan_not_found")
		default:
			httperr.Internal(c, "failed_to_update_plan", "failed_to_update_plan")
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *PlanHandler) SetActive(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	planIDParam := c.Param("id")
	planID64, err := strconv.ParseUint(planIDParam, 10, 64)
	if err != nil {
		httperr.BadRequest(c, "invalid_plan_id", "invalid_plan_id")
		return
	}

	var req struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", err.Error())
		return
	}

	if err := h.setActiveUC.Execute(c.Request.Context(), barbershopID, uint(planID64), req.Active); err != nil {
		switch {
		case errors.Is(err, subscription.ErrInvalidInput):
			httperr.BadRequest(c, "invalid_input", "invalid_input")
		case errors.Is(err, subscription.ErrSetPlanActiveNotFound):
			httperr.NotFound(c, "plan_not_found", "plan_not_found")
		default:
			httperr.Internal(c, "failed_to_update_plan", "failed_to_update_plan")
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *PlanHandler) List(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	plans, err := h.listUC.Execute(c.Request.Context(), barbershopID)
	if err != nil {
		switch {
		case errors.Is(err, subscription.ErrInvalidBarbershop):
			httperr.BadRequest(c, "invalid_barbershop", "invalid_barbershop")
		default:
			httperr.Internal(c, "failed_to_list", "failed_to_list")
		}
		return
	}

	c.JSON(http.StatusOK, plans)
}

func (h *PlanHandler) Delete(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	planIDParam := c.Param("id")
	planID64, err := strconv.ParseUint(planIDParam, 10, 64)
	if err != nil {
		httperr.BadRequest(c, "invalid_plan_id", "invalid_plan_id")
		return
	}

	if err := h.deleteUC.Execute(c.Request.Context(), barbershopID, uint(planID64)); err != nil {
		switch {
		case errors.Is(err, subscription.ErrInvalidInput):
			httperr.BadRequest(c, "invalid_input", "invalid_input")
		case errors.Is(err, subscription.ErrPlanNotFound):
			httperr.NotFound(c, "plan_not_found", "plan_not_found")
		case errors.Is(err, subscription.ErrPlanHasActiveSubscriptions):
			httperr.Write(c, http.StatusConflict, "plan_has_active_subscriptions", "plan_has_active_subscriptions")
		default:
			httperr.Internal(c, "failed_to_delete_plan", "failed_to_delete_plan")
		}
		return
	}

	c.Status(http.StatusNoContent)
}
