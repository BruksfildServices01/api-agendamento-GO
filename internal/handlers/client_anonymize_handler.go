package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	ucClient "github.com/BruksfildServices01/barber-scheduler/internal/usecase/client"
)

type ClientAnonymizeHandler struct {
	uc *ucClient.AnonymizeClient
}

func NewClientAnonymizeHandler(uc *ucClient.AnonymizeClient) *ClientAnonymizeHandler {
	return &ClientAnonymizeHandler{uc: uc}
}

// Anonymize executa a anonimização LGPD de um cliente.
// POST /api/me/clients/:id/anonymize
func (h *ClientAnonymizeHandler) Anonymize(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	userID       := c.MustGet(middleware.ContextUserID).(uint)

	clientID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || clientID == 0 {
		httperr.BadRequest(c, "bad_request", "invalid client id")
		return
	}

	if err := h.uc.Execute(c.Request.Context(), barbershopID, uint(clientID), userID, "lgpd_request"); err != nil {
		switch {
		case httperr.IsBusiness(err, "client_not_found"):
			httperr.NotFound(c, "client_not_found", "Cliente não encontrado.")
		case httperr.IsBusiness(err, "already_anonymized"):
			c.JSON(http.StatusConflict, gin.H{
				"error_code": "already_anonymized",
				"message":    "Este cliente já foi anonimizado.",
			})
		case httperr.IsBusiness(err, "active_subscription_exists"):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error_code": "active_subscription_exists",
				"message":    "Cancele a assinatura ativa antes de anonimizar.",
			})
		case httperr.IsBusiness(err, "future_appointments_exist"):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error_code": "future_appointments_exist",
				"message":    "Cancele os agendamentos futuros antes de anonimizar.",
			})
		default:
			httperr.Internal(c, "internal_error", "Erro ao processar anonimização.")
		}
		return
	}

	// Auditoria disparada após commit bem-sucedido da transação
	h.uc.DispatchAudit(barbershopID, uint(clientID), userID)

	c.Status(http.StatusNoContent)
}
