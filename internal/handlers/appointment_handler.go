package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type AppointmentHandler struct {
	db *gorm.DB
}

func NewAppointmentHandler(db *gorm.DB) *AppointmentHandler {
	return &AppointmentHandler{db: db}
}

// ---------- Requests ----------

type CreateAppointmentRequest struct {
	ClientName  string `json:"client_name" binding:"required"`
	ClientPhone string `json:"client_phone" binding:"required"`
	ClientEmail string `json:"client_email"`
	ProductID   uint   `json:"product_id" binding:"required"`
	Date        string `json:"date" binding:"required"`
	Time        string `json:"time" binding:"required"`
	Notes       string `json:"notes"`
}

// ---------- Helpers ----------

func parseDateTime(dateStr, timeStr string) (time.Time, error) {
	layout := "2006-01-02 15:04"
	return time.Parse(layout, dateStr+" "+timeStr)
}

func isInThePastOrTooSoon(start time.Time, minAdvanceMinutes int) bool {
	now := time.Now()
	if start.Before(now) {
		return true
	}
	minAllowed := now.Add(time.Duration(minAdvanceMinutes) * time.Minute)
	return start.Before(minAllowed)
}

func (h *AppointmentHandler) isWithinWorkingHours(barberID uint, start, end time.Time) (bool, error) {
	weekday := int(start.Weekday())

	var wh models.WorkingHours
	if err := h.db.
		Where("barber_id = ? AND weekday = ?", barberID, weekday).
		First(&wh).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}

	if !wh.Active {
		return false, nil
	}

	parseHM := func(s string) (time.Time, error) {
		if s == "" {
			return time.Time{}, nil
		}
		t, err := time.Parse("15:04", s)
		if err != nil {
			return time.Time{}, err
		}
		return time.Date(start.Year(), start.Month(), start.Day(),
			t.Hour(), t.Minute(), 0, 0, start.Location()), nil
	}

	workStart, err := parseHM(wh.StartTime)
	if err != nil {
		return false, err
	}
	workEnd, err := parseHM(wh.EndTime)
	if err != nil {
		return false, err
	}

	if (!workStart.IsZero() && start.Before(workStart)) ||
		(!workEnd.IsZero() && end.After(workEnd)) {
		return false, nil
	}

	if wh.LunchStart != "" && wh.LunchEnd != "" {
		lunchStart, err := parseHM(wh.LunchStart)
		if err != nil {
			return false, err
		}
		lunchEnd, err := parseHM(wh.LunchEnd)
		if err != nil {
			return false, err
		}
		if start.Before(lunchEnd) && end.After(lunchStart) {
			return false, nil
		}
	}

	return true, nil
}

// Verifica conflito com outros appointments do barbeiro
func (h *AppointmentHandler) hasConflict(barberID uint, start, end time.Time) (bool, error) {
	var count int64
	err := h.db.Model(&models.Appointment{}).
		Where("barber_id = ? AND status = ?", barberID, "scheduled").
		Where("start_time < ? AND ? < end_time", end, start).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ---------- Handlers ----------

func (h *AppointmentHandler) Create(c *gin.Context) {
	userIDVal, _ := c.Get(middleware.ContextUserID)
	barberID := userIDVal.(uint)

	barbershopIDVal, _ := c.Get(middleware.ContextBarbershopID)
	barbershopID := barbershopIDVal.(uint)

	var shop models.Barbershop
	if err := h.db.First(&shop, barbershopID).Error; err != nil {
		httperr.Internal(c, "failed_to_get_barbershop", "Erro ao buscar configurações da barbearia.")
		return
	}

	var req CreateAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "Dados inválidos na requisição.")
		return
	}

	start, err := parseDateTime(req.Date, req.Time)
	if err != nil {
		httperr.BadRequest(c, "invalid_date_or_time", "Data ou horário em formato inválido.")
		return
	}

	minAdvance := shop.MinAdvanceMinutes
	if minAdvance <= 0 {
		minAdvance = 120
	}
	if isInThePastOrTooSoon(start, minAdvance) {
		httperr.BadRequest(
			c,
			"too_soon_or_in_past",
			"Não é possível agendar para o passado ou com menos antecedência do que a configurada pela barbearia.",
		)
		return
	}

	var product models.BarberProduct
	if err := h.db.Where("id = ? AND barbershop_id = ?", req.ProductID, barbershopID).
		First(&product).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			httperr.BadRequest(c, "product_not_found", "Serviço não encontrado para esta barbearia.")
			return
		}
		httperr.Internal(c, "failed_to_get_product", "Erro ao buscar o serviço. Tente novamente mais tarde.")
		return
	}

	end := start.Add(time.Duration(product.DurationMin) * time.Minute)

	ok, err := h.isWithinWorkingHours(barberID, start, end)
	if err != nil {
		httperr.Internal(c, "failed_to_check_working_hours", "Erro ao validar horário de trabalho.")
		return
	}
	if !ok {
		httperr.BadRequest(c, "outside_working_hours", "O horário selecionado está fora do horário de atendimento.")
		return
	}

	conflict, err := h.hasConflict(barberID, start, end)
	if err != nil {
		httperr.Internal(c, "failed_to_check_conflicts", "Erro ao verificar conflitos de horário.")
		return
	}
	if conflict {
		httperr.BadRequest(c, "time_conflict", "Já existe um agendamento nesse horário.")
		return
	}

	var client models.Client
	if err := h.db.
		Where("barbershop_id = ? AND phone = ?", barbershopID, req.ClientPhone).
		First(&client).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			client = models.Client{
				BarbershopID: barbershopID,
				Name:         req.ClientName,
				Phone:        req.ClientPhone,
				Email:        req.ClientEmail,
			}
			if err := h.db.Create(&client).Error; err != nil {
				httperr.Internal(c, "failed_to_create_client", "Erro ao salvar o cliente. Tente novamente.")
				return
			}
		} else {
			httperr.Internal(c, "failed_to_get_client", "Erro ao buscar o cliente. Tente novamente.")
			return
		}
	}

	appointment := models.Appointment{
		BarbershopID:    barbershopID,
		BarberID:        barberID,
		ClientID:        client.ID,
		BarberProductID: product.ID,
		StartTime:       start,
		EndTime:         end,
		Status:          "scheduled",
		Notes:           req.Notes,
	}

	if err := h.db.Create(&appointment).Error; err != nil {
		httperr.Internal(c, "failed_to_create_appointment", "Erro ao criar o agendamento. Tente novamente.")
		return
	}

	if err := h.db.Preload("Client").
		Preload("BarberProduct").
		Preload("Barber").
		Preload("Barbershop").
		First(&appointment, appointment.ID).Error; err != nil {

		c.JSON(201, appointment)
		return
	}

	c.JSON(201, appointment)
}

func (h *AppointmentHandler) ListByDate(c *gin.Context) {
	userIDVal, _ := c.Get(middleware.ContextUserID)
	barberID := userIDVal.(uint)

	dateStr := c.Query("date")
	if dateStr == "" {
		httperr.BadRequest(c, "missing_date", "Parâmetro 'date' é obrigatório (YYYY-MM-DD).")
		return
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		httperr.BadRequest(c, "invalid_date", "Data em formato inválido. Use YYYY-MM-DD.")
		return
	}

	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
	endOfDay := startOfDay.Add(24 * time.Hour)

	var appointments []models.Appointment
	if err := h.db.
		Where("barber_id = ? AND start_time >= ? AND start_time < ?", barberID, startOfDay, endOfDay).
		Preload("Client").
		Preload("BarberProduct").
		Preload("Barber").
		Preload("Barbershop").
		Order("start_time ASC").
		Find(&appointments).Error; err != nil {

		httperr.Internal(c, "failed_to_list_appointments", "Erro ao listar os agendamentos.")
		return
	}

	c.JSON(200, appointments)
}

func (h *AppointmentHandler) Cancel(c *gin.Context) {
	userIDVal, _ := c.Get(middleware.ContextUserID)
	barberID := userIDVal.(uint)

	id := c.Param("id")

	var ap models.Appointment
	if err := h.db.
		Where("id = ? AND barber_id = ?", id, barberID).
		First(&ap).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			httperr.NotFound(c, "appointment_not_found", "Agendamento não encontrado.")
			return
		}
		httperr.Internal(c, "failed_to_get_appointment", "Erro ao buscar o agendamento.")
		return
	}

	if ap.Status == "cancelled" {
		httperr.BadRequest(c, "already_cancelled", "Este agendamento já foi cancelado.")
		return
	}
	if ap.Status == "completed" {
		httperr.BadRequest(c, "already_completed", "Este agendamento já foi concluído.")
		return
	}

	now := time.Now()
	ap.Status = "cancelled"
	ap.CancelledAt = &now

	if err := h.db.Save(&ap).Error; err != nil {
		httperr.Internal(c, "failed_to_cancel_appointment", "Erro ao cancelar o agendamento. Tente novamente.")
		return
	}

	c.JSON(200, ap)
}

func (h *AppointmentHandler) Complete(c *gin.Context) {
	userIDVal, _ := c.Get(middleware.ContextUserID)
	barberID := userIDVal.(uint)

	id := c.Param("id")

	var ap models.Appointment
	if err := h.db.
		Where("id = ? AND barber_id = ?", id, barberID).
		First(&ap).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			httperr.NotFound(c, "appointment_not_found", "Agendamento não encontrado.")
			return
		}
		httperr.Internal(c, "failed_to_get_appointment", "Erro ao buscar o agendamento.")
		return
	}

	if ap.Status == "cancelled" {
		httperr.BadRequest(c, "already_cancelled", "Este agendamento já foi cancelado.")
		return
	}
	if ap.Status == "completed" {
		httperr.BadRequest(c, "already_completed", "Este agendamento já foi concluído.")
		return
	}

	ap.Status = "completed"

	if err := h.db.Save(&ap).Error; err != nil {
		httperr.Internal(c, "failed_to_complete_appointment", "Erro ao concluir o agendamento. Tente novamente.")
		return
	}

	c.JSON(200, ap)
}
