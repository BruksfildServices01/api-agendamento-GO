package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	productDomain "github.com/BruksfildServices01/barber-scheduler/internal/domain/product"
	"github.com/BruksfildServices01/barber-scheduler/internal/dto"
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
	checkoutCartUC   *cartUC.CheckoutCart
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
	checkoutCartUC *cartUC.CheckoutCart,
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
		checkoutCartUC:      checkoutCartUC,
	}
}

////////////////////////////////////////////////////////
// PUBLIC SERVICES
////////////////////////////////////////////////////////

func (h *PublicHandler) ListServices(c *gin.Context) {
	shop, ok := h.getPublicBarbershop(c)
	if !ok {
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
// PUBLIC INFO
////////////////////////////////////////////////////////

func (h *PublicHandler) GetInfo(c *gin.Context) {
	shop, ok := h.getPublicBarbershop(c)
	if !ok {
		return
	}

	var workingHours []models.WorkingHours
	h.db.WithContext(c.Request.Context()).
		Where("barbershop_id = ? AND barber_id = 0", shop.ID).
		Order("weekday asc").
		Find(&workingHours)

	type whDto struct {
		Weekday    int    `json:"weekday"`
		StartTime  string `json:"start_time"`
		EndTime    string `json:"end_time"`
		LunchStart string `json:"lunch_start"`
		LunchEnd   string `json:"lunch_end"`
		Active     bool   `json:"active"`
	}

	wh := make([]whDto, len(workingHours))
	for i, w := range workingHours {
		wh[i] = whDto{
			Weekday:    w.Weekday,
			StartTime:  w.StartTime,
			EndTime:    w.EndTime,
			LunchStart: w.LunchStart,
			LunchEnd:   w.LunchEnd,
			Active:     w.Active,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       shop.ID,
		"name":     shop.Name,
		"slug":     shop.Slug,
		"phone":    shop.Phone,
		"address":  shop.Address,
		"timezone": shop.Timezone,
		"working_hours": wh,
	})
}

////////////////////////////////////////////////////////
// PUBLIC PRODUCTS
////////////////////////////////////////////////////////

func (h *PublicHandler) ListProducts(c *gin.Context) {
	shop, ok := h.getPublicBarbershop(c)
	if !ok {
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

	var req dto.PublicAddCartItemRequest
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

func (h *PublicHandler) CheckoutCart(c *gin.Context) {
	shop, ok := h.getPublicBarbershop(c)
	if !ok {
		return
	}

	cartKey := strings.TrimSpace(c.GetHeader("X-Cart-Key"))
	order, err := h.checkoutCartUC.Execute(
		c.Request.Context(),
		cartUC.CheckoutCartInput{
			CartKey:      cartKey,
			BarbershopID: shop.ID,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, cartUC.ErrCheckoutInvalidCartKey):
			httperr.BadRequest(c, "invalid_cart_key", "Carrinho inválido.")
		case errors.Is(err, cartUC.ErrCheckoutEmptyCart):
			httperr.BadRequest(c, "empty_cart", "Carrinho vazio.")
		case errors.Is(err, productDomain.ErrProductNotFound):
			httperr.BadRequest(c, "product_not_found", "Produto não encontrado.")
		case errors.Is(err, productDomain.ErrInsufficientStock):
			httperr.BadRequest(c, "insufficient_stock", "Estoque insuficiente.")
		default:
			httperr.Internal(c, "failed_to_checkout_cart", "Erro ao finalizar carrinho.")
		}
		return
	}

	resp := dto.PublicCheckoutResponseDTO{
		Order: dto.PublicCheckoutOrderDTO{
			OrderID:    order.ID,
			Status:     string(order.Status),
			TotalCents: order.TotalAmount,
			ItemsCount: len(order.Items),
		},
		NextStep: dto.PublicCheckoutNextStepDTO{
			Action:     "order_payment_required",
			Method:     "POST",
			PaymentURL: "/api/public/" + shop.Slug + "/orders/" + strconv.FormatUint(uint64(order.ID), 10) + "/payment/pix",
		},
	}

	c.JSON(http.StatusCreated, resp)
}

////////////////////////////////////////////////////////
// PUBLIC SERVICE SUGGESTION
////////////////////////////////////////////////////////

func (h *PublicHandler) GetServiceSuggestion(c *gin.Context) {
	shop, ok := h.getPublicBarbershop(c)
	if !ok {
		return
	}

	serviceID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || serviceID64 == 0 {
		httperr.BadRequest(c, "invalid_service_id", "Serviço inválido.")
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

	var suggestionDTO *dto.PublicServiceSuggestionDTO
	if suggestion != nil {
		suggestionDTO = &dto.PublicServiceSuggestionDTO{
			ServiceID: uint(serviceID64),
			Product: &dto.PublicSuggestedProductDTO{
				ProductID:   suggestion.Product.ID,
				Name:        suggestion.Product.Name,
				Description: suggestion.Product.Description,
				Category:    suggestion.Product.Category,
				PriceCents:  suggestion.Product.Price,
			},
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"barbershop": gin.H{
			"id":   shop.ID,
			"name": shop.Name,
			"slug": shop.Slug,
		},
		"service_id": serviceID64,
		"suggestion": suggestionDTO,
	})
}

////////////////////////////////////////////////////////
// AVAILABILITY (timezone-safe)
////////////////////////////////////////////////////////

func (h *PublicHandler) AvailabilityForClient(c *gin.Context) {
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

	shop, ok := h.getPublicBarbershop(c)
	if !ok {
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
	shop, ok := h.getPublicBarbershop(c)
	if !ok {
		return
	}

	var req dto.PublicCreateAppointmentRequest
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

	idempotencyKey := strings.TrimSpace(c.GetHeader("X-Idempotency-Key"))

	ap, err := h.createAppointment.Execute(
		c.Request.Context(),
		appointmentUC.CreatePrivateAppointmentInput{
			BarbershopID:   shop.ID,
			BarberID:       barber.ID,
			ClientName:     req.ClientName,
			ClientPhone:    req.ClientPhone,
			ClientEmail:    req.ClientEmail,
			ProductID:      req.ServiceID,
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

////////////////////////////////////////////////////////
// BARBERSHOP RESOLUTION HELPERS
////////////////////////////////////////////////////////

func (h *PublicHandler) getPublicBarbershop(c *gin.Context) (*models.Barbershop, bool) {
	slug := c.Param("slug")
	shop, err := h.getPublicBarbershopBySlug(c.Request.Context(), slug)
	if err != nil {
		httperr.Internal(c, "failed_to_load_barbershop", "Erro ao carregar barbearia.")
		return nil, false
	}
	if shop == nil {
		httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
		return nil, false
	}
	return shop, true
}

func (h *PublicHandler) GetBarbershopIDBySlug(c *gin.Context, slug string) (uint, error) {
	shop, err := h.getPublicBarbershopBySlug(c.Request.Context(), slug)
	if err != nil {
		return 0, err
	}
	if shop == nil {
		return 0, nil
	}
	return shop.ID, nil
}

func (h *PublicHandler) getPublicBarbershopBySlug(ctx context.Context, slug string) (*models.Barbershop, error) {
	var shop models.Barbershop
	err := h.db.WithContext(ctx).Where("slug = ?", slug).First(&shop).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &shop, nil
}
