package handlers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
)

// ======================================================
// HANDLER
// ======================================================

type AppointmentHandler struct {
	db    *gorm.DB
	audit *audit.Logger
}

func NewAppointmentHandler(db *gorm.DB) *AppointmentHandler {
	return &AppointmentHandler{
		db:    db,
		audit: audit.New(db),
	}
}

// ======================================================
// REQUESTS
// ======================================================

type CreateAppointmentRequest struct {
	ClientName  string `json:"client_name" binding:"required"`
	ClientPhone string `json:"client_phone" binding:"required"`
	ClientEmail string `json:"client_email"`
	ProductID   uint   `json:"product_id" binding:"required"`
	Date        string `json:"date" binding:"required"`
	Time        string `json:"time" binding:"required"`
	Notes       string `json:"notes"`
}

// ======================================================
// HELPERS
// ======================================================

func isInThePastOrTooSoon(
	shop *models.Barbershop,
	start time.Time,
	minAdvanceMinutes int,
) bool {
	now := timezone.NowIn(shop.Timezone)
	minAllowed := now.Add(time.Duration(minAdvanceMinutes) * time.Minute)
	return start.Before(minAllowed)
}

// ======================================================
// CREATE (FASE 6 + 7 + 9)
// ======================================================

func (h *AppointmentHandler) Create(c *gin.Context) {
	barberID := c.MustGet(middleware.ContextUserID).(uint)
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	var shop models.Barbershop
	if err := h.db.First(&shop, barbershopID).Error; err != nil {
		httperr.Internal(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	var req CreateAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "Dados inválidos.")
		return
	}

	start, err := parseDateTimeInShop(&shop, req.Date, req.Time)
	if err != nil {
		httperr.BadRequest(c, "invalid_date_or_time", "Data ou hora inválida.")
		return
	}

	minAdvance := shop.MinAdvanceMinutes
	if minAdvance <= 0 {
		minAdvance = 120
	}
	if isInThePastOrTooSoon(&shop, start, minAdvance) {
		httperr.BadRequest(c, "too_soon", "Horário inválido.")
		return
	}

	var product models.BarberProduct
	if err := h.db.
		Where("id = ? AND barbershop_id = ?", req.ProductID, barbershopID).
		First(&product).Error; err != nil {
		httperr.BadRequest(c, "product_not_found", "Serviço não encontrado.")
		return
	}

	end := start.Add(time.Duration(product.DurationMin) * time.Minute)

	ok, err := h.isWithinWorkingHours(&shop, barberID, start, end)
	if err != nil {
		httperr.Internal(c, "working_hours_error", "Erro ao validar horário.")
		return
	}
	if !ok {
		httperr.BadRequest(c, "outside_working_hours", "Fora do horário de atendimento.")
		return
	}

	var client models.Client
	if err := h.db.
		Where("barbershop_id = ? AND phone = ?", barbershopID, req.ClientPhone).
		First(&client).Error; err != nil {

		client = models.Client{
			BarbershopID: barbershopID,
			Name:         req.ClientName,
			Phone:        req.ClientPhone,
			Email:        req.ClientEmail,
		}
		h.db.Create(&client)
	}

	var created models.Appointment

	err = h.db.Transaction(func(tx *gorm.DB) error {

		var conflicts []models.Appointment
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where(
				"barber_id = ? AND status = ? AND start_time < ? AND end_time > ?",
				barberID, "scheduled", end, start,
			).
			Find(&conflicts).Error; err != nil {
			return err
		}

		if len(conflicts) > 0 {
			return httperr.ErrBusiness("time_conflict")
		}

		ap := models.Appointment{
			BarbershopID:    barbershopID,
			BarberID:        barberID,
			ClientID:        client.ID,
			BarberProductID: product.ID,
			StartTime:       start,
			EndTime:         end,
			Status:          "scheduled",
			Notes:           req.Notes,
		}

		if err := tx.Create(&ap).Error; err != nil {
			return err
		}

		created = ap
		return nil
	})

	if err != nil {
		if httperr.IsBusiness(err, "time_conflict") || httperr.IsExclusionConflict(err) {

			h.audit.Log(
				barbershopID,
				&barberID,
				"appointment_conflict",
				"appointment",
				nil,
				map[string]any{
					"start": start,
					"end":   end,
				},
			)

			httperr.BadRequest(c, "time_conflict", "Conflito de horário.")
			return
		}

		httperr.Internal(c, "failed_to_create_appointment", "Erro ao criar agendamento.")
		return
	}

	h.audit.Log(
		barbershopID,
		&barberID,
		"appointment_created",
		"appointment",
		&created.ID,
		nil,
	)

	c.JSON(201, created)
}

// ======================================================
// LIST
// ======================================================

func (h *AppointmentHandler) ListByDate(c *gin.Context) {
	barberID := c.MustGet(middleware.ContextUserID).(uint)
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	dateStr := c.Query("date")
	if dateStr == "" {
		httperr.BadRequest(c, "missing_date", "Data obrigatória.")
		return
	}

	var shop models.Barbershop
	if err := h.db.First(&shop, barbershopID).Error; err != nil {
		httperr.Internal(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	date, err := parseDateInShop(&shop, dateStr)
	if err != nil {
		httperr.BadRequest(c, "invalid_date", "Data inválida.")
		return
	}

	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	end := start.Add(24 * time.Hour)

	var aps []models.Appointment
	h.db.
		Preload("Client").
		Preload("BarberProduct").
		Where(
			"barber_id = ? AND start_time >= ? AND start_time < ?",
			barberID, start, end,
		).
		Order("start_time ASC").
		Find(&aps)

	c.JSON(200, aps)
}

// ======================================================
// COMPLETE
// ======================================================

func (h *AppointmentHandler) Complete(c *gin.Context) {
	barberID := c.MustGet(middleware.ContextUserID).(uint)
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	id := c.Param("id")

	var shop models.Barbershop
	if err := h.db.First(&shop, barbershopID).Error; err != nil {
		httperr.Internal(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	var ap models.Appointment
	if err := h.db.Where("id = ? AND barber_id = ?", id, barberID).First(&ap).Error; err != nil {
		httperr.NotFound(c, "appointment_not_found", "Agendamento não encontrado.")
		return
	}

	if ap.Status != "scheduled" {
		httperr.BadRequest(c, "invalid_state", "Agendamento não pode ser concluído.")
		return
	}

	now := timezone.NowIn(shop.Timezone)
	ap.Status = "completed"
	ap.CompletedAt = &now

	h.db.Save(&ap)

	h.audit.Log(
		barbershopID,
		&barberID,
		"appointment_completed",
		"appointment",
		&ap.ID,
		nil,
	)

	c.JSON(200, ap)
}

// ======================================================
// CANCEL
// ======================================================

func (h *AppointmentHandler) Cancel(c *gin.Context) {
	barberID := c.MustGet(middleware.ContextUserID).(uint)
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)
	id := c.Param("id")

	var shop models.Barbershop
	if err := h.db.First(&shop, barbershopID).Error; err != nil {
		httperr.Internal(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	var ap models.Appointment
	if err := h.db.Where("id = ? AND barber_id = ?", id, barberID).First(&ap).Error; err != nil {
		httperr.NotFound(c, "appointment_not_found", "Agendamento não encontrado.")
		return
	}

	if ap.Status != "scheduled" {
		httperr.BadRequest(c, "invalid_state", "Agendamento não pode ser cancelado.")
		return
	}

	now := timezone.NowIn(shop.Timezone)
	ap.Status = "cancelled"
	ap.CancelledAt = &now

	h.db.Save(&ap)

	h.audit.Log(
		barbershopID,
		&barberID,
		"appointment_cancelled",
		"appointment",
		&ap.ID,
		nil,
	)

	c.JSON(200, ap)
}

// ======================================================
// WORKING HOURS + ALMOÇO
// ======================================================

func (h *AppointmentHandler) isWithinWorkingHours(
	shop *models.Barbershop,
	barberID uint,
	start time.Time,
	end time.Time,
) (bool, error) {

	weekday := int(start.Weekday())
	loc := start.Location()

	var wh models.WorkingHours
	if err := h.db.
		Where("barber_id = ? AND weekday = ?", barberID, weekday).
		First(&wh).Error; err != nil {
		return false, nil
	}

	if !wh.Active || wh.StartTime == "" || wh.EndTime == "" {
		return false, nil
	}

	parseHM := func(hm string) time.Time {
		t, _ := time.Parse("15:04", hm)
		return time.Date(start.Year(), start.Month(), start.Day(), t.Hour(), t.Minute(), 0, 0, loc)
	}

	workStart := parseHM(wh.StartTime)
	workEnd := parseHM(wh.EndTime)

	if start.Before(workStart) || end.After(workEnd) {
		return false, nil
	}

	if wh.LunchStart != "" && wh.LunchEnd != "" {
		lunchStart := parseHM(wh.LunchStart)
		lunchEnd := parseHM(wh.LunchEnd)
		if start.Before(lunchEnd) && end.After(lunchStart) {
			return false, nil
		}
	}

	return true, nil
}

// ======================================================
// LIST BY MONTH
// ======================================================

func (h *AppointmentHandler) ListByMonth(c *gin.Context) {
	barberID := c.MustGet(middleware.ContextUserID).(uint)
	barbershopID := c.MustGet(middleware.ContextBarbershopID).(uint)

	yearStr := c.Query("year")
	monthStr := c.Query("month")

	if yearStr == "" || monthStr == "" {
		httperr.BadRequest(c, "missing_year_or_month", "Ano e mês são obrigatórios.")
		return
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 2000 || year > 2100 {
		httperr.BadRequest(c, "invalid_year", "Ano inválido.")
		return
	}

	month, err := strconv.Atoi(monthStr)
	if err != nil || month < 1 || month > 12 {
		httperr.BadRequest(c, "invalid_month", "Mês inválido.")
		return
	}

	var shop models.Barbershop
	if err := h.db.First(&shop, barbershopID).Error; err != nil {
		httperr.Internal(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	loc := timezone.Location(shop.Timezone)
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)

	var appointments []models.Appointment
	h.db.
		Preload("Client").
		Preload("BarberProduct").
		Where(
			"barber_id = ? AND start_time >= ? AND start_time < ?",
			barberID, start, end,
		).
		Order("start_time ASC").
		Find(&appointments)

	c.JSON(200, gin.H{
		"year":         year,
		"month":        month,
		"appointments": appointments,
	})
}
