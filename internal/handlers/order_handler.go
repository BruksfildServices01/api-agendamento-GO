package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	productDomain "github.com/BruksfildServices01/barber-scheduler/internal/domain/product"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	ucOrder "github.com/BruksfildServices01/barber-scheduler/internal/usecase/order"
)

type OrderHandler struct {
	createUC *ucOrder.CreateOrder
}

func NewOrderHandler(createUC *ucOrder.CreateOrder) *OrderHandler {
	return &OrderHandler{createUC: createUC}
}

// ================================
// Request / Response DTOs
// ================================

type CreateOrderRequest struct {
	Items []CreateOrderItemRequest `json:"items" binding:"required"`
}

type CreateOrderItemRequest struct {
	ProductID uint `json:"product_id" binding:"required"`
	Quantity  int  `json:"quantity" binding:"required"`
}

type OrderResponse struct {
	ID           uint                `json:"id"`
	BarbershopID uint                `json:"barbershop_id"`
	Type         string              `json:"type"`
	Status       string              `json:"status"`
	TotalAmount  int64               `json:"total_amount"`
	Items        []OrderItemResponse `json:"items"`
}

type OrderItemResponse struct {
	ItemID    uint   `json:"item_id"`
	ItemName  string `json:"item_name"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
	Total     int64  `json:"total"`
}

// ================================
// POST /api/me/orders
// ================================

func (h *OrderHandler) Create(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_barbershop"})
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}

	items := make([]ucOrder.CreateOrderItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, ucOrder.CreateOrderItemInput{
			ProductID: it.ProductID,
			Quantity:  it.Quantity,
		})
	}

	order, err := h.createUC.Execute(
		c.Request.Context(),
		ucOrder.CreateOrderInput{
			BarbershopID: barbershopID, // 🔒 tenant vem do JWT (não do body)
			Items:        items,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, productDomain.ErrProductNotFound):
			c.JSON(http.StatusBadRequest, gin.H{"error": "product_not_found"})
			return
		case errors.Is(err, productDomain.ErrInsufficientStock):
			c.JSON(http.StatusConflict, gin.H{"error": "insufficient_stock"})
			return
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_create_order"})
			return
		}
	}

	resp := OrderResponse{
		ID:           order.ID,
		BarbershopID: order.BarbershopID,
		Type:         string(order.Type),
		Status:       string(order.Status),
		TotalAmount:  order.TotalAmount,
		Items:        make([]OrderItemResponse, 0, len(order.Items)),
	}

	for _, it := range order.Items {
		resp.Items = append(resp.Items, OrderItemResponse{
			ItemID:    it.ItemID,
			ItemName:  it.ItemName,
			Quantity:  it.Quantity,
			UnitPrice: it.UnitPrice,
			Total:     it.Total,
		})
	}

	c.JSON(http.StatusCreated, resp)
}
