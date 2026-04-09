package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	orderDomain "github.com/BruksfildServices01/barber-scheduler/internal/domain/order"
	productDomain "github.com/BruksfildServices01/barber-scheduler/internal/domain/product"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/httpresp"
	infraRepo "github.com/BruksfildServices01/barber-scheduler/internal/infra/repository"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	ucOrder "github.com/BruksfildServices01/barber-scheduler/internal/usecase/order"
)

type OrderHandler struct {
	createUC    *ucOrder.CreateOrder
	getUC       *ucOrder.GetOrder
	listAdminUC *ucOrder.ListOrdersAdmin
	orderRepo   *infraRepo.OrderGormRepository
}

func NewOrderHandler(
	createUC *ucOrder.CreateOrder,
	getUC *ucOrder.GetOrder,
	listAdminUC *ucOrder.ListOrdersAdmin,
	orderRepo *infraRepo.OrderGormRepository,
) *OrderHandler {
	return &OrderHandler{
		createUC:    createUC,
		getUC:       getUC,
		listAdminUC: listAdminUC,
		orderRepo:   orderRepo,
	}
}

type CreateOrderRequest struct {
	ClientID *uint                    `json:"client_id,omitempty"`
	Items    []CreateOrderItemRequest `json:"items" binding:"required"`
}

type CreateOrderItemRequest struct {
	ProductID uint `json:"product_id" binding:"required"`
	Quantity  int  `json:"quantity" binding:"required"`
}

type OrderResponse struct {
	ID             uint                `json:"id"`
	BarbershopID   uint                `json:"barbershop_id"`
	ClientID       *uint               `json:"client_id,omitempty"`
	Type           string              `json:"type"`
	Status         string              `json:"status"`
	SubtotalAmount int64               `json:"subtotal_amount"`
	DiscountAmount int64               `json:"discount_amount"`
	TotalAmount    int64               `json:"total_amount"`
	Items          []OrderItemResponse `json:"items"`
}

type OrderItemResponse struct {
	ProductID           uint   `json:"product_id"`
	ProductNameSnapshot string `json:"product_name_snapshot"`
	Quantity            int    `json:"quantity"`
	UnitPrice           int64  `json:"unit_price"`
	LineTotal           int64  `json:"line_total"`
}

type RichOrderClientInfo struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone,omitempty"`
	Email string `json:"email,omitempty"`
}

type RichOrderServiceInfo struct {
	Name          string `json:"name"`
	PaymentMethod string `json:"payment_method,omitempty"`
	AppointmentID *uint  `json:"appointment_id,omitempty"`
}

type RichOrderResponse struct {
	ID          uint                 `json:"id"`
	Status      string               `json:"status"`
	OrderSource string               `json:"order_source"` // "suggestion" | "standalone"
	Client      *RichOrderClientInfo `json:"client,omitempty"`
	Service     *RichOrderServiceInfo `json:"service,omitempty"`
	SubtotalAmount int64             `json:"subtotal_amount"`
	DiscountAmount int64             `json:"discount_amount"`
	TotalAmount    int64             `json:"total_amount"`
	Items          []OrderItemResponse `json:"items"`
	CreatedAt      string             `json:"created_at"`
}

func (h *OrderHandler) Create(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		httperr.Unauthorized(c, "invalid_barbershop", "Barbershop inválida.")
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "Payload inválido.")
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
			BarbershopID: barbershopID,
			ClientID:     req.ClientID,
			Items:        items,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, productDomain.ErrProductNotFound):
			httperr.BadRequest(c, "product_not_found", "Produto não encontrado.")
			return
		default:
			httperr.Internal(c, "failed_to_create_order", "Falha ao criar pedido.")
			return
		}
	}

	c.JSON(http.StatusCreated, toOrderResponse(order))
}

func (h *OrderHandler) GetByID(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		httperr.Unauthorized(c, "invalid_barbershop", "Barbershop inválida.")
		return
	}

	orderID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || orderID64 == 0 {
		httperr.BadRequest(c, "invalid_order_id", "ID do pedido inválido.")
		return
	}

	rich, err := h.orderRepo.GetRichByID(c.Request.Context(), barbershopID, uint(orderID64))
	if err != nil {
		httperr.Internal(c, "failed_to_get_order", "Falha ao buscar pedido.")
		return
	}
	if rich == nil {
		httperr.NotFound(c, "order_not_found", "Pedido não encontrado.")
		return
	}

	resp := RichOrderResponse{
		ID:             rich.ID,
		Status:         rich.Status,
		OrderSource:    rich.OrderSource,
		SubtotalAmount: rich.SubtotalAmount,
		DiscountAmount: rich.DiscountAmount,
		TotalAmount:    rich.TotalAmount,
		CreatedAt:      rich.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Items:          make([]OrderItemResponse, 0, len(rich.Items)),
	}

	if rich.ClientID != nil && rich.ClientName != "" {
		resp.Client = &RichOrderClientInfo{
			ID:    *rich.ClientID,
			Name:  rich.ClientName,
			Phone: rich.ClientPhone,
			Email: rich.ClientEmail,
		}
	}

	if rich.OrderSource == "suggestion" && rich.ServiceName != "" {
		resp.Service = &RichOrderServiceInfo{
			Name:          rich.ServiceName,
			PaymentMethod: rich.PaymentMethod,
			AppointmentID: rich.AppointmentID,
		}
	}

	for _, it := range rich.Items {
		resp.Items = append(resp.Items, OrderItemResponse{
			ProductID:           it.ProductID,
			ProductNameSnapshot: it.ProductNameSnapshot,
			Quantity:            it.Quantity,
			UnitPrice:           it.UnitPrice,
			LineTotal:           it.LineTotal,
		})
	}

	httpresp.OK(c, resp)
}

func (h *OrderHandler) List(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		httperr.Unauthorized(c, "invalid_barbershop", "Barbershop inválida.")
		return
	}

	page, err := parsePositiveIntDefault(c.Query("page"), 1)
	if err != nil {
		httperr.BadRequest(c, "invalid_page", "Parâmetro page inválido.")
		return
	}

	limit, err := parsePositiveIntDefault(c.Query("limit"), 10)
	if err != nil {
		httperr.BadRequest(c, "invalid_limit", "Parâmetro limit inválido.")
		return
	}

	var statusPtr *string
	status := strings.TrimSpace(c.Query("status"))
	if status != "" {
		statusPtr = &status
	}

	result, err := h.listAdminUC.Execute(
		c.Request.Context(),
		ucOrder.ListOrdersAdminInput{
			BarbershopID: barbershopID,
			Status:       statusPtr,
			Page:         page,
			Limit:        limit,
			SortBy:       c.Query("sort"),
			SortOrder:    c.Query("order"),
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, ucOrder.ErrInvalidPage):
			httperr.BadRequest(c, "invalid_page", "Parâmetro page inválido.")
			return
		case errors.Is(err, ucOrder.ErrInvalidLimit):
			httperr.BadRequest(c, "invalid_limit", "Parâmetro limit inválido.")
			return
		case errors.Is(err, ucOrder.ErrInvalidSortField):
			httperr.BadRequest(c, "invalid_sort", "Campo de ordenação inválido.")
			return
		case errors.Is(err, ucOrder.ErrInvalidSortOrder):
			httperr.BadRequest(c, "invalid_order", "Direção de ordenação inválida.")
			return
		default:
			httperr.Internal(c, "failed_to_list_orders", "Falha ao listar pedidos.")
			return
		}
	}

	httpresp.OK(c, result)
}

func toOrderResponse(order *orderDomain.Order) OrderResponse {
	resp := OrderResponse{
		ID:             order.ID,
		BarbershopID:   order.BarbershopID,
		ClientID:       order.ClientID,
		Type:           string(order.Type),
		Status:         string(order.Status),
		SubtotalAmount: order.SubtotalAmount,
		DiscountAmount: order.DiscountAmount,
		TotalAmount:    order.TotalAmount,
		Items:          make([]OrderItemResponse, 0, len(order.Items)),
	}

	for _, it := range order.Items {
		resp.Items = append(resp.Items, OrderItemResponse{
			ProductID:           it.ProductID,
			ProductNameSnapshot: it.ProductNameSnapshot,
			Quantity:            it.Quantity,
			UnitPrice:           it.UnitPrice,
			LineTotal:           it.LineTotal,
		})
	}

	return resp
}

func parsePositiveIntDefault(raw string, def int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return def, nil
	}

	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if v < 1 {
		return 0, errors.New("must be positive")
	}

	return v, nil
}
