package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/servicesuggestion"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	uc "github.com/BruksfildServices01/barber-scheduler/internal/usecase/servicesuggestion"
)

type ServiceSuggestionHandler struct {
	setUC    *uc.SetServiceSuggestion
	getUC    *uc.GetServiceSuggestion
	removeUC *uc.RemoveServiceSuggestion
}

func NewServiceSuggestionHandler(
	setUC *uc.SetServiceSuggestion,
	getUC *uc.GetServiceSuggestion,
	removeUC *uc.RemoveServiceSuggestion,
) *ServiceSuggestionHandler {
	return &ServiceSuggestionHandler{
		setUC:    setUC,
		getUC:    getUC,
		removeUC: removeUC,
	}
}

type SetServiceSuggestionRequest struct {
	ProductID uint `json:"product_id" binding:"required"`
}

func (h *ServiceSuggestionHandler) Set(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		httperr.Unauthorized(c, "invalid_barbershop", "invalid_barbershop")
		return
	}

	serviceID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || serviceID64 == 0 {
		httperr.BadRequest(c, "invalid_service_id", "invalid_service_id")
		return
	}

	var req SetServiceSuggestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "invalid_request")
		return
	}

	err = h.setUC.Execute(
		c.Request.Context(),
		uc.SetServiceSuggestionInput{
			BarbershopID: barbershopID,
			ServiceID:    uint(serviceID64),
			ProductID:    req.ProductID,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidContext):
			httperr.BadRequest(c, "invalid_context", "invalid_context")
		case errors.Is(err, domain.ErrServiceNotFound):
			httperr.NotFound(c, "service_not_found", "service_not_found")
		case errors.Is(err, domain.ErrProductNotFound):
			httperr.NotFound(c, "product_not_found", "product_not_found")
		case errors.Is(err, domain.ErrInvalidSuggestedProduct):
			httperr.BadRequest(c, "invalid_suggested_product", "invalid_suggested_product")
		default:
			httperr.Internal(c, "failed_to_set_service_suggestion", "failed_to_set_service_suggestion")
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ServiceSuggestionHandler) Get(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		httperr.Unauthorized(c, "invalid_barbershop", "invalid_barbershop")
		return
	}

	serviceID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || serviceID64 == 0 {
		httperr.BadRequest(c, "invalid_service_id", "invalid_service_id")
		return
	}

	suggestion, err := h.getUC.Execute(
		c.Request.Context(),
		uc.GetServiceSuggestionInput{
			BarbershopID: barbershopID,
			ServiceID:    uint(serviceID64),
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidContext):
			httperr.BadRequest(c, "invalid_context", "invalid_context")
		default:
			httperr.Internal(c, "failed_to_get_service_suggestion", "failed_to_get_service_suggestion")
		}
		return
	}

	if suggestion == nil {
		c.Status(http.StatusNoContent)
		return
	}

	c.JSON(http.StatusOK, suggestion)
}

func (h *ServiceSuggestionHandler) Remove(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		httperr.Unauthorized(c, "invalid_barbershop", "invalid_barbershop")
		return
	}

	serviceID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || serviceID64 == 0 {
		httperr.BadRequest(c, "invalid_service_id", "invalid_service_id")
		return
	}

	err = h.removeUC.Execute(
		c.Request.Context(),
		uc.RemoveServiceSuggestionInput{
			BarbershopID: barbershopID,
			ServiceID:    uint(serviceID64),
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidContext):
			httperr.BadRequest(c, "invalid_context", "invalid_context")
		default:
			httperr.Internal(c, "failed_to_remove_service_suggestion", "failed_to_remove_service_suggestion")
		}
		return
	}

	c.Status(http.StatusNoContent)
}
