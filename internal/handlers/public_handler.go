package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/httpresp"
	infraRepo "github.com/BruksfildServices01/barber-scheduler/internal/infra/repository"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
	appointmentUC "github.com/BruksfildServices01/barber-scheduler/internal/usecase/appointment"
	cartUC "github.com/BruksfildServices01/barber-scheduler/internal/usecase/cart"
	productUC "github.com/BruksfildServices01/barber-scheduler/internal/usecase/product"
	serviceUC "github.com/BruksfildServices01/barber-scheduler/internal/usecase/service"
	serviceSuggestionUC "github.com/BruksfildServices01/barber-scheduler/internal/usecase/servicesuggestion"
)

////////////////////////////////////////////////////////
// HANDLER
////////////////////////////////////////////////////////

type PublicHandler struct {
	db *gorm.DB

	createAppointment   *appointmentUC.CreatePrivateAppointment
	listPublicServices  *serviceUC.ListPublicServices
	listPublicProducts  *productUC.ListPublicProducts
	getPublicSuggestion *serviceSuggestionUC.GetPublicServiceSuggestion

	getCartUC        *cartUC.GetCart
	addCartItemUC    *cartUC.AddItem
	removeCartItemUC *cartUC.RemoveItem
}

func NewPublicHandler(
	db *gorm.DB,
	createAppointment *appointmentUC.CreatePrivateAppointment,
	listPublicServices *serviceUC.ListPublicServices,
	listPublicProducts *productUC.ListPublicProducts,
	getPublicSuggestion *serviceSuggestionUC.GetPublicServiceSuggestion,
	getCartUC *cartUC.GetCart,
	addCartItemUC *cartUC.AddItem,
	removeCartItemUC *cartUC.RemoveItem,
) *PublicHandler {
	return &PublicHandler{
		db:                  db,
		createAppointment:   createAppointment,
		listPublicServices:  listPublicServices,
		listPublicProducts:  listPublicProducts,
		getPublicSuggestion: getPublicSuggestion,
		getCartUC:           getCartUC,
		addCartItemUC:       addCartItemUC,
		removeCartItemUC:    removeCartItemUC,
	}
}

////////////////////////////////////////////////////////
// DTOs
////////////////////////////////////////////////////////

type PublicCreateAppointmentRequest struct {
	ClientName  string `json:"client_name" binding:"required"`
	ClientPhone string `json:"client_phone" binding:"required"`
	ClientEmail string `json:"client_email"`
	ProductID   uint   `json:"product_id" binding:"required"`
	Date        string `json:"date" binding:"required"` // YYYY-MM-DD
	Time        string `json:"time" binding:"required"` // HH:mm
	Notes       string `json:"notes"`
}

type PublicAddCartItemRequest struct {
	ProductID uint `json:"product_id" binding:"required"`
	Quantity  int  `json:"quantity" binding:"required"`
}

////////////////////////////////////////////////////////
// PUBLIC SERVICES
////////////////////////////////////////////////////////

func (h *PublicHandler) ListServices(c *gin.Context) {
	slug := c.Param("slug")

	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	category := strings.TrimSpace(strings.ToLower(c.Query("category")))
	query := strings.TrimSpace(c.Query("query"))

	services, err := h.listPublicServices.Execute(
		c.Request.Context(),
		serviceUC.ListPublicServicesInput{
			BarbershopID: shop.ID,
			Category:     category,
			Query:        query,
		},
	)
	if err != nil {
		httperr.Internal(c, "failed_to_list_services", "Erro ao listar serviços.")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"barbershop": gin.H{
			"id":   shop.ID,
			"name": shop.Name,
			"slug": shop.Slug,
		},
		"services": services,
	})
}

////////////////////////////////////////////////////////
// PUBLIC PRODUCTS
////////////////////////////////////////////////////////

func (h *PublicHandler) ListProducts(c *gin.Context) {
	slug := c.Param("slug")

	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	category := strings.TrimSpace(strings.ToLower(c.Query("category")))
	query := strings.TrimSpace(c.Query("query"))

	products, err := h.listPublicProducts.Execute(
		c.Request.Context(),
		productUC.ListPublicProductsInput{
			BarbershopID: shop.ID,
			Category:     category,
			Query:        query,
		},
	)
	if err != nil {
		httperr.Internal(c, "failed_to_list_products", "Erro ao listar produtos.")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"barbershop": gin.H{
			"id":   shop.ID,
			"name": shop.Name,
			"slug": shop.Slug,
		},
		"products": products,
		"total":    len(products),
	})
}

////////////////////////////////////////////////////////
// PUBLIC CART
////////////////////////////////////////////////////////

func (h *PublicHandler) GetCart(c *gin.Context) {
	shop, ok := h.getPublicBarbershop(c)
	if !ok {
		return
	}

	cartKey := strings.TrimSpace(c.GetHeader("X-Cart-Key"))
	result, err := h.getCartUC.Execute(
		c.Request.Context(),
		cartUC.GetCartInput{
			CartKey:      cartKey,
			BarbershopID: shop.ID,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, cartUC.ErrGetCartInvalidKey):
			httperr.BadRequest(c, "invalid_cart_key", "Carrinho inválido.")
		default:
			httperr.Internal(c, "failed_to_get_cart", "Erro ao consultar carrinho.")
		}
		return
	}

	httpresp.OK(c, result)
}

func (h *PublicHandler) AddCartItem(c *gin.Context) {
	shop, ok := h.getPublicBarbershop(c)
	if !ok {
		return
	}

	cartKey := strings.TrimSpace(c.GetHeader("X-Cart-Key"))
	if cartKey == "" {
		httperr.BadRequest(c, "invalid_cart_key", "Carrinho inválido.")
		return
	}

	var req PublicAddCartItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "Dados inválidos.")
		return
	}

	result, err := h.addCartItemUC.Execute(
		c.Request.Context(),
		cartUC.AddItemInput{
			CartKey:      cartKey,
			BarbershopID: shop.ID,
			ProductID:    req.ProductID,
			Quantity:     req.Quantity,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, cartUC.ErrInvalidCartKey):
			httperr.BadRequest(c, "invalid_cart_key", "Carrinho inválido.")
		case errors.Is(err, cartUC.ErrInvalidProductID):
			httperr.BadRequest(c, "invalid_product_id", "Produto inválido.")
		case errors.Is(err, cartUC.ErrInvalidQuantity):
			httperr.BadRequest(c, "invalid_quantity", "Quantidade inválida.")
		case errors.Is(err, cartUC.ErrProductNotFound):
			httperr.BadRequest(c, "product_not_found", "Produto não encontrado.")
		case errors.Is(err, cartUC.ErrProductUnavailable):
			httperr.BadRequest(c, "product_unavailable", "Produto indisponível para venda online.")
		default:
			httperr.Internal(c, "failed_to_add_cart_item", "Erro ao adicionar item no carrinho.")
		}
		return
	}

	httpresp.OK(c, result)
}

func (h *PublicHandler) RemoveCartItem(c *gin.Context) {
	shop, ok := h.getPublicBarbershop(c)
	if !ok {
		return
	}

	cartKey := strings.TrimSpace(c.GetHeader("X-Cart-Key"))
	productID64, err := strconv.ParseUint(c.Param("productId"), 10, 64)
	if err != nil || productID64 == 0 {
		httperr.BadRequest(c, "invalid_product_id", "Produto inválido.")
		return
	}

	result, err := h.removeCartItemUC.Execute(
		c.Request.Context(),
		cartUC.RemoveItemInput{
			CartKey:      cartKey,
			BarbershopID: shop.ID,
			ProductID:    uint(productID64),
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, cartUC.ErrRemoveItemInvalidKey):
			httperr.BadRequest(c, "invalid_cart_key", "Carrinho inválido.")
		case errors.Is(err, cartUC.ErrRemoveItemInvalidProduct):
			httperr.BadRequest(c, "invalid_product_id", "Produto inválido.")
		default:
			httperr.Internal(c, "failed_to_remove_cart_item", "Erro ao remover item do carrinho.")
		}
		return
	}

	httpresp.OK(c, result)
}

////////////////////////////////////////////////////////
// PUBLIC SERVICE SUGGESTION
////////////////////////////////////////////////////////

func (h *PublicHandler) GetServiceSuggestion(c *gin.Context) {
	slug := c.Param("slug")

	serviceID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || serviceID64 == 0 {
		httperr.BadRequest(c, "invalid_service_id", "Serviço inválido.")
		return
	}

	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	suggestion, err := h.getPublicSuggestion.Execute(
		c.Request.Context(),
		serviceSuggestionUC.GetPublicServiceSuggestionInput{
			BarbershopID: shop.ID,
			ServiceID:    uint(serviceID64),
		},
	)
	if err != nil {
		httperr.Internal(c, "failed_to_get_service_suggestion", "Erro ao buscar sugestão do serviço.")
		return
	}

	if suggestion == nil {
		c.JSON(http.StatusOK, gin.H{
			"service_id": serviceID64,
			"suggestion": nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"service_id": serviceID64,
		"suggestion": suggestion,
	})
}

////////////////////////////////////////////////////////
// AVAILABILITY (timezone-safe)
////////////////////////////////////////////////////////

func (h *PublicHandler) AvailabilityForClient(c *gin.Context) {
	slug := c.Param("slug")
	dateStr := strings.TrimSpace(c.Query("date"))
	productIDStr := strings.TrimSpace(c.Query("product_id"))

	if dateStr == "" || productIDStr == "" {
		httperr.BadRequest(c, "missing_params", "Data e serviço obrigatórios.")
		return
	}

	productID, err := strconv.ParseUint(productIDStr, 10, 64)
	if err != nil || productID == 0 {
		httperr.BadRequest(c, "invalid_product_id", "Serviço inválido.")
		return
	}

	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	var barber models.User
	if err := h.db.
		Where("barbershop_id = ? AND role = ?", shop.ID, "owner").
		First(&barber).Error; err != nil {
		httperr.BadRequest(c, "barber_not_found", "Barbeiro não encontrado.")
		return
	}

	loc := timezone.Location(shop.Timezone)

	parsed, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		httperr.BadRequest(c, "invalid_date", "Data inválida.")
		return
	}

	date := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, loc)

	repo := infraRepo.NewAppointmentGormRepository(h.db)
	uc := appointmentUC.NewGetAvailability(repo)

	slots, err := uc.Execute(
		c.Request.Context(),
		domain.AvailabilityInput{
			BarbershopID: shop.ID,
			BarberID:     barber.ID,
			ProductID:    uint(productID),
			Date:         date,
		},
	)
	if err != nil {
		if httperr.IsBusiness(err, "product_not_found") {
			httperr.BadRequest(c, "product_not_found", "Serviço inválido.")
			return
		}
		httperr.Internal(c, "availability_failed", "Erro ao calcular horários.")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"date":     dateStr,
		"timezone": shop.Timezone,
		"slots":    slots,
	})
}

func mapPublicCreateErrors(c *gin.Context, err error) {
	switch {
	case httperr.IsBusiness(err, "barbershop_not_found"):
		httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")

	case httperr.IsBusiness(err, "duplicate_request"):
		httperr.Write(
			c,
			http.StatusConflict,
			"duplicate_request",
			"Requisição duplicada. Aguarde antes de tentar novamente.",
		)

	case httperr.IsBusiness(err, "invalid_date_or_time"):
		httperr.BadRequest(c, "invalid_date_or_time", "Data ou hora inválida.")

	case httperr.IsBusiness(err, "too_soon"):
		httperr.BadRequest(c, "too_soon", "Horário inválido.")

	case httperr.IsBusiness(err, "product_not_found"):
		httperr.BadRequest(c, "product_not_found", "Serviço não encontrado.")

	case httperr.IsBusiness(err, "outside_working_hours"):
		httperr.BadRequest(c, "outside_working_hours", "Fora do horário de atendimento.")

	case httperr.IsBusiness(err, "time_conflict"):
		httperr.BadRequest(c, "time_conflict", "Conflito de horário.")

	default:
		httperr.Internal(c, "failed_to_create_appointment", "Erro ao criar agendamento.")
	}
}

////////////////////////////////////////////////////////
// CREATE APPOINTMENT (PUBLIC → reuses private usecase)
////////////////////////////////////////////////////////

func (h *PublicHandler) CreateAppointment(c *gin.Context) {
	slug := c.Param("slug")

	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	var req PublicCreateAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "Dados inválidos.")
		return
	}

	var barber models.User
	if err := h.db.
		Where("barbershop_id = ? AND role = ?", shop.ID, "owner").
		First(&barber).Error; err != nil {
		httperr.BadRequest(c, "barber_not_found", "Barbeiro não encontrado.")
		return
	}

	idempotencyKey := c.GetHeader("X-Idempotency-Key")

	ap, err := h.createAppointment.Execute(
		c.Request.Context(),
		appointmentUC.CreatePrivateAppointmentInput{
			BarbershopID:   shop.ID,
			BarberID:       barber.ID,
			ClientName:     req.ClientName,
			ClientPhone:    req.ClientPhone,
			ClientEmail:    req.ClientEmail,
			ProductID:      req.ProductID,
			Date:           req.Date,
			Time:           req.Time,
			Notes:          req.Notes,
			IdempotencyKey: idempotencyKey,
		},
	)
	if err != nil {
		mapPublicCreateErrors(c, err)
		return
	}

	c.JSON(http.StatusCreated, ap)
}

func (h *PublicHandler) getPublicBarbershop(c *gin.Context) (*models.Barbershop, bool) {
	slug := c.Param("slug")

	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
		return nil, false
	}

	return &shop, true
}
