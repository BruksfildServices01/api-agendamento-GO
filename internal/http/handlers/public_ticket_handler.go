package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	domainTicket "github.com/BruksfildServices01/barber-scheduler/internal/domain/ticket"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	ucTicket "github.com/BruksfildServices01/barber-scheduler/internal/usecase/ticket"
)

type PublicTicketHandler struct {
	view       *ucTicket.ViewTicket
	cancel     *ucTicket.CancelViaTicket
	reschedule *ucTicket.RescheduleViaTicket
}

func NewPublicTicketHandler(
	view *ucTicket.ViewTicket,
	cancel *ucTicket.CancelViaTicket,
	reschedule *ucTicket.RescheduleViaTicket,
) *PublicTicketHandler {
	return &PublicTicketHandler{
		view:       view,
		cancel:     cancel,
		reschedule: reschedule,
	}
}

func (h *PublicTicketHandler) View(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		httperr.BadRequest(c, "invalid_token", "Token inválido.")
		return
	}

	dto, err := h.view.Execute(c.Request.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, domainTicket.ErrTicketNotFound):
			httperr.NotFound(c, "ticket_not_found", "Ticket não encontrado.")
		case errors.Is(err, domainTicket.ErrTokenExpired):
			httperr.Write(c, http.StatusGone, "ticket_expired", "Ticket expirado.")
		default:
			httperr.Internal(c, "ticket_view_failed", "Erro ao carregar ticket.")
		}
		return
	}

	c.JSON(http.StatusOK, dto)
}

func (h *PublicTicketHandler) Cancel(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		httperr.BadRequest(c, "invalid_token", "Token inválido.")
		return
	}

	err := h.cancel.Execute(c.Request.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, domainTicket.ErrTicketNotFound):
			httperr.NotFound(c, "ticket_not_found", "Ticket não encontrado.")
		case errors.Is(err, domainTicket.ErrTokenExpired):
			httperr.Write(c, http.StatusGone, "ticket_expired", "Ticket expirado.")
		case errors.Is(err, ucTicket.ErrCannotCancel):
			httperr.Write(c, http.StatusUnprocessableEntity, "cannot_cancel", "O agendamento não pode ser cancelado.")
		case errors.Is(err, ucTicket.ErrCancellationWindowClosed):
			httperr.Write(c, http.StatusUnprocessableEntity, "cancellation_window_closed", "O prazo para cancelamento passou.")
		default:
			httperr.Internal(c, "ticket_cancel_failed", "Erro ao cancelar agendamento.")
		}
		return
	}

	c.Status(http.StatusNoContent)
}

type rescheduleRequest struct {
	Date string `json:"date"`
	Time string `json:"time"`
}

func (h *PublicTicketHandler) Reschedule(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		httperr.BadRequest(c, "invalid_token", "Token inválido.")
		return
	}

	var req rescheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "Dados inválidos.")
		return
	}

	newToken, err := h.reschedule.Execute(c.Request.Context(), token, req.Date, req.Time)
	if err != nil {
		switch {
		case errors.Is(err, domainTicket.ErrTicketNotFound):
			httperr.NotFound(c, "ticket_not_found", "Ticket não encontrado.")
		case errors.Is(err, domainTicket.ErrTokenExpired):
			httperr.Write(c, http.StatusGone, "ticket_expired", "Ticket expirado.")
		case errors.Is(err, ucTicket.ErrRescheduleNotAllowed):
			httperr.Write(c, http.StatusUnprocessableEntity, "reschedule_not_allowed", "Remarcação não permitida.")
		case errors.Is(err, ucTicket.ErrRescheduleWindowClosed):
			httperr.Write(c, http.StatusUnprocessableEntity, "reschedule_window_closed", "O prazo para remarcação passou.")
		case errors.Is(err, ucTicket.ErrTooSoon):
			httperr.Write(c, http.StatusUnprocessableEntity, "too_soon", "O novo horário é muito próximo.")
		case errors.Is(err, ucTicket.ErrTimeConflict):
			httperr.Write(c, http.StatusConflict, "time_conflict", "Conflito de horário.")
		case errors.Is(err, ucTicket.ErrOutsideWorkingHours):
			httperr.Write(c, http.StatusUnprocessableEntity, "outside_working_hours", "Fora do horário de atendimento.")
		default:
			httperr.Internal(c, "ticket_reschedule_failed", "Erro ao remarcar agendamento.")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   newToken,
		"message": "Agendamento remarcado com sucesso.",
	})
}
