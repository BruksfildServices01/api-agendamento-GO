package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type PublicHandler struct {
	db    *gorm.DB
	audit *audit.Logger
}

func NewPublicHandler(db *gorm.DB) *PublicHandler {
	return &PublicHandler{
		db:    db,
		audit: audit.New(db),
	}
}

// ======================================================
// STRUCTS
// ======================================================

type TimeSlot struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type PublicCreateAppointmentRequest struct {
	ClientName  string `json:"client_name" binding:"required"`
	ClientPhone string `json:"client_phone" binding:"required"`
	ClientEmail string `json:"client_email"`
	ProductID   uint   `json:"product_id" binding:"required"`
	Date        string `json:"date" binding:"required"`
	Time        string `json:"time" binding:"required"`
	Notes       string `json:"notes"`
}

// ======================================================
// PRODUCTS
// ======================================================

func (h *PublicHandler) ListProducts(c *gin.Context) {
	slug := c.Param("slug")

	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	category := strings.ToLower(strings.TrimSpace(c.Query("category")))
	query := strings.ToLower(strings.TrimSpace(c.Query("query")))

	q := h.db.Where("barbershop_id = ? AND active = true", shop.ID)

	if category != "" {
		q = q.Where("LOWER(category) = ?", category)
	}
	if query != "" {
		like := "%" + query + "%"
		q = q.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", like, like)
	}

	var products []models.BarberProduct
	if err := q.Order("id ASC").Find(&products).Error; err != nil {
		httperr.Internal(c, "failed_to_list_products", "Erro ao listar produtos.")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"barbershop": shop,
		"products":   products,
	})
}

// ======================================================
// AVAILABILITY
// ======================================================

func (h *PublicHandler) AvailabilityForClient(c *gin.Context) {
	h.Availability(c)
}

func (h *PublicHandler) Availability(c *gin.Context) {
	slug := c.Param("slug")
	dateStr := c.Query("date")
	productID := c.Query("product_id")

	if dateStr == "" || productID == "" {
		httperr.BadRequest(c, "missing_params", "Data e serviço obrigatórios.")
		return
	}

	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
		return
	}

	date, err := parseDateInShop(&shop, dateStr)
	if err != nil {
		httperr.BadRequest(c, "invalid_date", "Data inválida.")
		return
	}

	var product models.BarberProduct
	if err := h.db.
		Where("id = ? AND barbershop_id = ? AND active = true", productID, shop.ID).
		First(&product).Error; err != nil {
		httperr.BadRequest(c, "product_not_found", "Serviço inválido.")
		return
	}

	var barber models.User
	if err := h.db.
		Where("barbershop_id = ? AND role = ?", shop.ID, "owner").
		First(&barber).Error; err != nil {
		httperr.BadRequest(c, "barber_not_found", "Barbeiro não encontrado.")
		return
	}

	slots, err := h.generateAvailabilitySlots(&shop, barber.ID, date, &product)
	if err != nil {
		httperr.Internal(c, "failed_to_generate_slots", "Erro ao gerar horários.")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"date":  dateStr,
		"slots": slots,
	})
}

// ======================================================
// CREATE APPOINTMENT (PUBLIC)
// ======================================================

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

	start, err := parseDateTimeInShop(&shop, req.Date, req.Time)
	if err != nil {
		httperr.BadRequest(c, "invalid_date_or_time", "Data ou hora inválida.")
		return
	}

	var product models.BarberProduct
	if err := h.db.
		Where("id = ? AND barbershop_id = ? AND active = true", req.ProductID, shop.ID).
		First(&product).Error; err != nil {
		httperr.BadRequest(c, "product_not_found", "Serviço inválido.")
		return
	}

	var barber models.User
	if err := h.db.
		Where("barbershop_id = ? AND role = ?", shop.ID, "owner").
		First(&barber).Error; err != nil {
		httperr.BadRequest(c, "barber_not_found", "Barbeiro não encontrado.")
		return
	}

	end := start.Add(time.Duration(product.DurationMin) * time.Minute)

	var client models.Client
	if err := h.db.
		Where("barbershop_id = ? AND phone = ?", shop.ID, req.ClientPhone).
		First(&client).Error; err != nil {

		client = models.Client{
			BarbershopID: shop.ID,
			Name:         req.ClientName,
			Phone:        req.ClientPhone,
			Email:        req.ClientEmail,
		}
		h.db.Create(&client)
	}

	ap := models.Appointment{
		BarbershopID:    shop.ID,
		BarberID:        barber.ID,
		ClientID:        client.ID,
		BarberProductID: product.ID,
		StartTime:       start,
		EndTime:         end,
		Status:          "scheduled",
		Notes:           req.Notes,
	}

	if err := h.db.Create(&ap).Error; err != nil {
		httperr.Internal(c, "failed_to_create_appointment", "Erro ao criar agendamento.")
		return
	}

	c.JSON(http.StatusCreated, ap)
}

// ======================================================
// SLOTS
// ======================================================

func (h *PublicHandler) generateAvailabilitySlots(
	shop *models.Barbershop,
	barberID uint,
	date time.Time,
	product *models.BarberProduct,
) ([]TimeSlot, error) {

	weekday := int(date.Weekday())

	var wh models.WorkingHours
	if err := h.db.
		Where("barber_id = ? AND weekday = ?", barberID, weekday).
		First(&wh).Error; err != nil {
		return []TimeSlot{}, nil
	}

	if !wh.Active || wh.StartTime == "" || wh.EndTime == "" {
		return []TimeSlot{}, nil
	}

	loc := date.Location()

	parseHM := func(hm string) time.Time {
		t, _ := time.Parse("15:04", hm)
		return time.Date(
			date.Year(), date.Month(), date.Day(),
			t.Hour(), t.Minute(), 0, 0,
			loc,
		)
	}

	dayStart := parseHM(wh.StartTime)
	dayEnd := parseHM(wh.EndTime)

	hasLunch := wh.LunchStart != "" && wh.LunchEnd != ""
	var lunchStart, lunchEnd time.Time
	if hasLunch {
		lunchStart = parseHM(wh.LunchStart)
		lunchEnd = parseHM(wh.LunchEnd)
	}

	startOfDay := time.Date(
		date.Year(), date.Month(), date.Day(),
		0, 0, 0, 0,
		loc,
	)
	endOfDay := startOfDay.Add(24 * time.Hour)

	var appointments []models.Appointment
	h.db.
		Where(
			"barber_id = ? AND status = ? AND start_time >= ? AND start_time < ?",
			barberID, "scheduled", startOfDay, endOfDay,
		).
		Find(&appointments)

	slotDuration := time.Duration(product.DurationMin) * time.Minute
	var slots []TimeSlot

	for cur := dayStart; cur.Add(slotDuration).Before(dayEnd) || cur.Add(slotDuration).Equal(dayEnd); cur = cur.Add(slotDuration) {

		slotStart := cur
		slotEnd := cur.Add(slotDuration)

		if hasLunch {
			if slotStart.Before(lunchEnd) && slotEnd.After(lunchStart) {
				continue
			}
		}

		conflict := false
		for _, ap := range appointments {
			if slotStart.Before(ap.EndTime) && slotEnd.After(ap.StartTime) {
				conflict = true
				break
			}
		}

		if conflict {
			continue
		}

		slots = append(slots, TimeSlot{
			Start: slotStart.Format("15:04"),
			End:   slotEnd.Format("15:04"),
		})
	}

	return slots, nil
}
