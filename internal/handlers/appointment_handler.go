package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/BruksfildServices01/barber-scheduler/internal/dto"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/httpresp"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/usecase/appointment"
)

////////////////////////////////////////////////////////
// HANDLER
////////////////////////////////////////////////////////

type AppointmentHandler struct {
	createUC    *appointment.CreatePrivateAppointment
	completeUC  *appointment.CompleteAppointment
	cancelUC    *appointment.CancelAppointment
	listByDate  *appointment.ListAppointmentsByDate
	listByMonth *appointment.ListAppointmentsByMonth
	noShow      *appointment.MarkAppointmentNoShow
}

type CompleteAppointmentRequest struct {
	FinalAmountCents      *int64 `json:"final_amount_cents"`
	OperationalNote       string `json:"operational_note"`
	ConfirmNormalCharging bool   `json:"confirm_normal_charging"`
}

func NewAppointmentHandler(
	create *appointment.CreatePrivateAppointment,
	complete *appointment.CompleteAppointment,
	cancel *appointment.CancelAppointment,
	noShow *appointment.MarkAppointmentNoShow,
	listByDate *appointment.ListAppointmentsByDate,
	listByMonth *appointment.ListAppointmentsByMonth,
) *AppointmentHandler {
	return &AppointmentHandler{
		createUC:    create,
		completeUC:  complete,
		cancelUC:    cancel,
		noShow:      noShow,
		listByDate:  listByDate,
		listByMonth: listByMonth,
	}
}

////////////////////////////////////////////////////////
// REQUESTS
////////////////////////////////////////////////////////

type CreateAppointmentRequest struct {
	ClientName  string `json:"client_name" binding:"required"`
	ClientPhone string `json:"client_phone" binding:"required"`
	ClientEmail string `json:"client_email"`
	ProductID   uint   `json:"product_id" binding:"required"`
	Date        string `json:"date" binding:"required"` // YYYY-MM-DD
	Time        string `json:"time" binding:"required"` // HH:mm
	Notes       string `json:"notes"`
}

////////////////////////////////////////////////////////
// CREATE
////////////////////////////////////////////////////////

func (h *AppointmentHandler) Create(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	barberID := c.MustGet(middleware.ContextUserID).(uint)

	var req CreateAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "Dados inválidos.")
		return
	}

	ap, err := h.createUC.Execute(
		c.Request.Context(),
		appointment.CreatePrivateAppointmentInput{
			BarbershopID: barbershopID,
			BarberID:     barberID,
			ClientName:   req.ClientName,
			ClientPhone:  req.ClientPhone,
			ClientEmail:  req.ClientEmail,
			ProductID:    req.ProductID,
			Date:         req.Date,
			Time:         req.Time,
			Notes:        req.Notes,
		},
	)

	if err != nil {
		mapCreateErrors(c, err)
		return
	}

	c.JSON(http.StatusCreated, ap)
}

////////////////////////////////////////////////////////
// COMPLETE
////////////////////////////////////////////////////////

func (h *AppointmentHandler) Complete(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	barberID := c.MustGet(middleware.ContextUserID).(uint)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httperr.BadRequest(c, "invalid_id", "ID inválido.")
		return
	}

	var req CompleteAppointmentRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			httperr.BadRequest(c, "invalid_request", "Dados inválidos.")
			return
		}
	}

	ap, closure, consumeResult, err := h.completeUC.Execute(
		c.Request.Context(),
		appointment.CompleteAppointmentInput{
			BarbershopID:          barbershopID,
			BarberID:              barberID,
			AppointmentID:         uint(id),
			FinalAmountCents:      req.FinalAmountCents,
			OperationalNote:       req.OperationalNote,
			ConfirmNormalCharging: req.ConfirmNormalCharging,
		},
	)
	if err != nil {
		switch {
		case httperr.IsBusiness(err, "appointment_not_found"):
			httperr.NotFound(c, "appointment_not_found", "Agendamento não encontrado.")

		case httperr.IsBusiness(err, "invalid_barbershop"):
			httperr.BadRequest(c, "invalid_barbershop", "Barbearia inválida.")

		case httperr.IsBusiness(err, "invalid_final_amount"):
			httperr.BadRequest(c, "invalid_final_amount", "Valor final inválido.")

		case httperr.IsBusiness(err, "invalid_state"):
			httperr.BadRequest(c, "invalid_state", "Agendamento não pode ser concluído.")

		case httperr.IsBusiness(err, "appointment_payment_not_found"):
			httperr.BadRequest(
				c,
				"appointment_payment_not_found",
				"Pagamento obrigatório ainda não foi gerado.",
			)

		case httperr.IsBusiness(err, "appointment_payment_not_paid"):
			httperr.BadRequest(
				c,
				"appointment_payment_not_paid",
				"Pagamento obrigatório ainda não foi confirmado.",
			)

		default:
			httperr.Internal(c, "complete_failed", "Erro ao concluir agendamento.")
		}
		return
	}

	var subscription *dto.CompleteAppointmentSubscriptionDTO
	if consumeResult != nil {
		subscription = &dto.CompleteAppointmentSubscriptionDTO{
			ConsumeStatus: string(consumeResult.Status),
			PlanID:        consumeResult.PlanID,
		}
	}

	operational := dto.CompleteAppointmentOperationalDTO{}

	if closure != nil {
		operational.ServiceID = closure.ServiceID
		operational.ServiceName = closure.ServiceName
		operational.ReferenceAmountCents = closure.ReferenceAmountCents
		operational.FinalAmountCents = closure.FinalAmountCents
		operational.OperationalNote = closure.OperationalNote
		operational.SubscriptionCovered = closure.SubscriptionCovered
		operational.RequiresNormalCharging = closure.RequiresNormalCharging
		operational.ConfirmNormalCharging = closure.ConfirmNormalCharging

		if closure.SubscriptionConsumeStatus != nil {
			operational.SubscriptionConsumeStatus = *closure.SubscriptionConsumeStatus
		}
	}

	c.JSON(http.StatusOK, dto.CompleteAppointmentResponse{
		Appointment:  ap,
		Subscription: subscription,
		Operational:  operational,
	})
}

////////////////////////////////////////////////////////
// CANCEL
////////////////////////////////////////////////////////

func (h *AppointmentHandler) Cancel(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	barberID := c.MustGet(middleware.ContextUserID).(uint)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httperr.BadRequest(c, "invalid_id", "ID inválido.")
		return
	}

	ap, err := h.cancelUC.Execute(
		c.Request.Context(),
		barbershopID,
		barberID,
		uint(id),
	)

	if err != nil {
		switch {
		case httperr.IsBusiness(err, "appointment_not_found"):
			httperr.NotFound(c, "appointment_not_found", "Agendamento não encontrado.")
		case httperr.IsBusiness(err, "invalid_state"):
			httperr.BadRequest(c, "invalid_state", "Agendamento não pode ser cancelado.")
		default:
			httperr.Internal(c, "cancel_failed", "Erro ao cancelar agendamento.")
		}
		return
	}

	c.JSON(http.StatusOK, ap)
}

////////////////////////////////////////////////////////
// LIST BY DATE
////////////////////////////////////////////////////////

func (h *AppointmentHandler) ListByDate(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	barberID := c.MustGet(middleware.ContextUserID).(uint)

	dateStr := c.Query("date")
	if dateStr == "" {
		httperr.BadRequest(c, "missing_date", "Data obrigatória.")
		return
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		httperr.BadRequest(c, "invalid_date", "Data inválida.")
		return
	}

	list, err := h.listByDate.Execute(
		c.Request.Context(),
		barbershopID,
		barberID,
		date,
	)
	if err != nil {
		httperr.Internal(c, "failed_to_list_appointments", "Erro ao listar agendamentos.")
		return
	}

	httpresp.List(c, list)
}

func (h *AppointmentHandler) ListByMonth(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	barberID := c.MustGet(middleware.ContextUserID).(uint)

	year, err := strconv.Atoi(c.Query("year"))
	if err != nil {
		httperr.BadRequest(c, "invalid_year", "Ano inválido.")
		return
	}

	month, err := strconv.Atoi(c.Query("month"))
	if err != nil {
		httperr.BadRequest(c, "invalid_month", "Mês inválido.")
		return
	}

	list, err := h.listByMonth.Execute(
		c.Request.Context(),
		barbershopID,
		barberID,
		year,
		month,
	)
	if err != nil {
		httperr.Internal(c, "failed_to_list_appointments", "Erro ao listar agendamentos.")
		return
	}

	httpresp.List(c, list)
}

////////////////////////////////////////////////////////
// HELPERS
////////////////////////////////////////////////////////

func mapCreateErrors(c *gin.Context, err error) {
	switch {
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

func (h *AppointmentHandler) MarkNoShow(c *gin.Context) {
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	barberID := c.MustGet(middleware.ContextUserID).(uint)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httperr.BadRequest(c, "invalid_id", "ID inválido.")
		return
	}

	err = h.noShow.Execute(
		c.Request.Context(),
		barbershopID,
		barberID,
		uint(id),
	)

	if err != nil {
		switch {
		case httperr.IsBusiness(err, "appointment_not_found"):
			httperr.NotFound(c, "appointment_not_found", "Agendamento não encontrado.")

		case httperr.IsBusiness(err, "invalid_state"):
			httperr.BadRequest(c, "invalid_state", "Agendamento não pode ser marcado como falta.")

		default:
			httperr.Internal(c, "no_show_failed", "Erro ao marcar falta.")
		}
		return
	}

	c.Status(http.StatusNoContent)
}
