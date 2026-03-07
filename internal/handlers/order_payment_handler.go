package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	domainOrder "github.com/BruksfildServices01/barber-scheduler/internal/domain/order"
	"github.com/BruksfildServices01/barber-scheduler/internal/dto"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

// contrato mínimo (seu OrderGormRepository já atende)
type OrderRepository interface {
	GetByID(ctx context.Context, barbershopID uint, id uint) (*domainOrder.Order, error)
}

type OrderPaymentHandler struct {
	createPixForOrder *ucPayment.CreatePixPaymentForOrder
	orderRepo         OrderRepository
}

func NewOrderPaymentHandler(
	createPixForOrder *ucPayment.CreatePixPaymentForOrder,
	orderRepo OrderRepository,
) *OrderPaymentHandler {
	return &OrderPaymentHandler{
		createPixForOrder: createPixForOrder,
		orderRepo:         orderRepo,
	}
}

// POST /api/me/orders/:id/payment/pix
func (h *OrderPaymentHandler) Create(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_barbershop"})
		return
	}

	orderID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || orderID64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_order_id"})
		return
	}
	orderID := uint(orderID64)

	order, err := h.orderRepo.GetByID(c.Request.Context(), barbershopID, orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_load_order"})
		return
	}
	if order == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order_not_found"})
		return
	}

	out, err := h.createPixForOrder.Execute(c.Request.Context(), order)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, dto.PixResponse{
		PaymentID: out.PaymentID,
		Pix: dto.PixPayload{
			TxID:      out.TxID,
			QRCode:    out.QRCode,
			ExpiresAt: out.ExpiresAt,
		},
	})
}
