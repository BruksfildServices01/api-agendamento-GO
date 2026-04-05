package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	domainService "github.com/BruksfildServices01/barber-scheduler/internal/domain/service"
	"github.com/BruksfildServices01/barber-scheduler/internal/dto"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	ucCart "github.com/BruksfildServices01/barber-scheduler/internal/usecase/cart"
	ucPublic "github.com/BruksfildServices01/barber-scheduler/internal/usecase/public"
)

type PublicCheckoutHandler struct {
	public *PublicHandler
	uc     *ucPublic.OrchestratedCheckout
}

func NewPublicCheckoutHandler(
	public *PublicHandler,
	uc *ucPublic.OrchestratedCheckout,
) *PublicCheckoutHandler {
	return &PublicCheckoutHandler{
		public: public,
		uc:     uc,
	}
}

func (h *PublicCheckoutHandler) Checkout(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		httperr.BadRequest(c, "invalid_slug", "Slug inválido.")
		return
	}

	barbershopID, err := h.public.GetBarbershopIDBySlug(c, slug)
	if err != nil {
		httperr.Internal(c, "failed_to_load_barbershop", "Erro ao carregar barbearia.")
		return
	}
	if barbershopID == 0 {
		httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	var req dto.PublicOrchestratedCheckoutRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "Dados inválidos.")
		return
	}
	req.IdempotencyKey = strings.TrimSpace(c.GetHeader("X-Idempotency-Key"))
	out, err := h.uc.Execute(c.Request.Context(), barbershopID, req)
	if err != nil {
		switch {
		case errors.Is(err, domainService.ErrServiceNotFound):
			httperr.BadRequest(c, "service_not_found", "Serviço não encontrado.")

		case httperr.IsBusiness(err, "duplicate_request"):
			httperr.Write(c, http.StatusConflict, "duplicate_request", "Requisição duplicada. Aguarde antes de tentar novamente.")

		case httperr.IsBusiness(err, "invalid_date_or_time"):
			httperr.BadRequest(c, "invalid_date_or_time", "Data ou hora inválida.")

		case httperr.IsBusiness(err, "too_soon"):
			httperr.BadRequest(c, "too_soon", "Horário inválido.")

		case httperr.IsBusiness(err, "outside_working_hours"):
			httperr.BadRequest(c, "outside_working_hours", "Fora do horário de atendimento.")

		case httperr.IsBusiness(err, "time_conflict"):
			httperr.BadRequest(c, "time_conflict", "Conflito de horário.")

		case httperr.IsBusiness(err, "product_not_found"):
			httperr.BadRequest(c, "product_not_found", "Produto não encontrado no carrinho.")

		case errors.Is(err, ucCart.ErrCheckoutInvalidCartKey):
			httperr.BadRequest(c, "invalid_cart_key", "Carrinho inválido.")

		case errors.Is(err, ucCart.ErrCheckoutEmptyCart):
			httperr.BadRequest(c, "empty_cart", "Carrinho vazio.")

		default:
			httperr.Internal(c, "public_orchestrated_checkout_failed", "Erro ao finalizar checkout.")
		}
		return
	}

	if out.Payments.AppointmentPaymentRequired && out.Appointment != nil {
		out.NextURLs.AppointmentPixURL = "/api/public/" + slug + "/appointments/" + strconv.FormatUint(uint64(out.Appointment.ID), 10) + "/payment/pix"
	}

	if out.Payments.OrderPaymentRequired && out.Order != nil {
		out.NextURLs.OrderPixURL = "/api/public/" + slug + "/orders/" + strconv.FormatUint(uint64(out.Order.ID), 10) + "/payment/pix"
	}

	c.JSON(http.StatusCreated, out)
}
