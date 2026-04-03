package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	domainOrder "github.com/BruksfildServices01/barber-scheduler/internal/domain/order"
	"github.com/BruksfildServices01/barber-scheduler/internal/dto"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

type OrderRepository interface {
	GetByID(ctx context.Context, barbershopID uint, id uint) (*domainOrder.Order, error)
}

type OrderPaymentHandler struct {
	db                *gorm.DB
	createPixForOrder *ucPayment.CreatePixPaymentForOrder
	orderRepo         OrderRepository
}

func NewOrderPaymentHandler(
	db *gorm.DB,
	createPixForOrder *ucPayment.CreatePixPaymentForOrder,
	orderRepo OrderRepository,
) *OrderPaymentHandler {
	return &OrderPaymentHandler{
		db:                db,
		createPixForOrder: createPixForOrder,
		orderRepo:         orderRepo,
	}
}

// POST /api/me/orders/:id/payment/pix
func (h *OrderPaymentHandler) Create(c *gin.Context) {
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

	h.handlePixForOrder(c, barbershopID, uint(orderID64))
}

// POST /api/public/:slug/orders/:id/payment/pix
func (h *OrderPaymentHandler) CreatePublic(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		httperr.BadRequest(c, "invalid_slug", "Slug inválido.")
		return
	}

	var shop models.Barbershop
	if err := h.db.
		WithContext(c.Request.Context()).
		Select("id").
		Where("slug = ?", slug).
		First(&shop).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
			return
		}

		httperr.Internal(c, "failed_to_load_barbershop", "Erro ao carregar barbearia.")
		return
	}

	orderID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || orderID64 == 0 {
		httperr.BadRequest(c, "invalid_order_id", "ID do pedido inválido.")
		return
	}

	h.handlePixForOrder(c, shop.ID, uint(orderID64))
}

func (h *OrderPaymentHandler) handlePixForOrder(c *gin.Context, barbershopID, orderID uint) {
	order, err := h.orderRepo.GetByID(c.Request.Context(), barbershopID, orderID)
	if err != nil {
		httperr.Internal(c, "failed_to_load_order", "Falha ao carregar pedido.")
		return
	}
	if order == nil {
		httperr.NotFound(c, "order_not_found", "Pedido não encontrado.")
		return
	}

	out, err := h.createPixForOrder.Execute(c.Request.Context(), order)
	if err != nil {
		httperr.BadRequest(c, "failed_to_create_pix_for_order", err.Error())
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
