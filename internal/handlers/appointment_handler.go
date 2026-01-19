package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

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
}

func NewAppointmentHandler(
	createUC *appointment.CreatePrivateAppointment,
	completeUC *appointment.CompleteAppointment,
	cancelUC *appointment.CancelAppointment,
	listByDate *appointment.ListAppointmentsByDate,
	listByMonth *appointment.ListAppointmentsByMonth,
) *AppointmentHandler {
	return &AppointmentHandler{
		createUC:    createUC,
		completeUC:  completeUC,
		cancelUC:    cancelUC,
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
	barberID := c.MustGet(middleware.ContextUserID).(uint)
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

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
	barberID := c.MustGet(middleware.ContextUserID).(uint)
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httperr.BadRequest(c, "invalid_id", "ID inválido.")
		return
	}

	ap, err := h.completeUC.Execute(
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
			httperr.BadRequest(c, "invalid_state", "Agendamento não pode ser concluído.")
		default:
			httperr.Internal(c, "complete_failed", "Erro ao concluir agendamento.")
		}
		return
	}

	c.JSON(http.StatusOK, ap)
}

////////////////////////////////////////////////////////
// CANCEL
////////////////////////////////////////////////////////

func (h *AppointmentHandler) Cancel(c *gin.Context) {
	barberID := c.MustGet(middleware.ContextUserID).(uint)
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

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
	barberID := c.MustGet(middleware.ContextUserID).(uint)
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

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
		barberID,
		barbershopID,
		date,
	)
	if err != nil {
		httperr.Internal(c, "failed_to_list_appointments", "Erro ao listar agendamentos.")
		return
	}

	httpresp.List(c, list)
}

func (h *AppointmentHandler) ListByMonth(c *gin.Context) {
	barberID := c.MustGet(middleware.ContextUserID).(uint)
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

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
		barberID,
		barbershopID,
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
