package handlers

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type PublicHandler struct {
	db *gorm.DB
}

func NewPublicHandler(db *gorm.DB) *PublicHandler {
	return &PublicHandler{db: db}
}

// ---------- Structs auxiliares ----------

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

const publicMinAdvanceMinutes = 120

func publicIsInThePastOrTooSoon(start time.Time) bool {
	now := time.Now()
	if start.Before(now) {
		return true
	}
	minAllowed := now.Add(time.Duration(publicMinAdvanceMinutes) * time.Minute)
	return start.Before(minAllowed)
}

// ---------- Produtos públicos ----------
func (h *PublicHandler) ListProducts(c *gin.Context) {
	slug := c.Param("slug")

	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
			return
		}
		httperr.Internal(c, "failed_to_get_barbershop", "Erro ao buscar a barbearia. Tente novamente.")
		return
	}

	category := strings.ToLower(strings.TrimSpace(c.Query("category")))
	minPriceStr := c.Query("min_price")
	maxPriceStr := c.Query("max_price")
	query := strings.ToLower(strings.TrimSpace(c.Query("query")))
	sort := strings.ToLower(strings.TrimSpace(c.Query("sort")))

	q := h.db.Where("barbershop_id = ? AND active = true", shop.ID)

	if category != "" {
		q = q.Where("LOWER(category) = ?", category)
	}

	if minPriceStr != "" {
		if minPrice, err := strconv.ParseFloat(minPriceStr, 64); err == nil {
			q = q.Where("price >= ?", minPrice)
		} else {
			httperr.BadRequest(c, "invalid_min_price", "Valor de min_price inválido.")
			return
		}
	}

	if maxPriceStr != "" {
		if maxPrice, err := strconv.ParseFloat(maxPriceStr, 64); err == nil {
			q = q.Where("price <= ?", maxPrice)
		} else {
			httperr.BadRequest(c, "invalid_max_price", "Valor de max_price inválido.")
			return
		}
	}

	if query != "" {
		like := "%" + query + "%"
		q = q.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", like, like)
	}

	orderClause := "id ASC"
	switch sort {
	case "price_asc":
		orderClause = "price ASC"
	case "price_desc":
		orderClause = "price DESC"
	case "duration_asc":
		orderClause = "duration_min ASC"
	case "duration_desc":
		orderClause = "duration_min DESC"
	}

	var products []models.BarberProduct
	if err := q.
		Order(orderClause).
		Find(&products).Error; err != nil {

		httperr.Internal(c, "failed_to_list_products", "Erro ao listar os serviços. Tente novamente.")
		return
	}

	c.JSON(200, gin.H{
		"barbershop": gin.H{
			"id":      shop.ID,
			"name":    shop.Name,
			"slug":    shop.Slug,
			"phone":   shop.Phone,
			"address": shop.Address,
		},
		"products": products,
	})
}

// ---------- Disponibilidade pública ----------

func (h *PublicHandler) Availability(c *gin.Context) {
	slug := c.Param("slug")
	dateStr := c.Query("date")
	productID := c.Query("product_id")

	if dateStr == "" || productID == "" {
		httperr.BadRequest(c, "missing_date_or_product_id", "Parâmetros 'date' (YYYY-MM-DD) e 'product_id' são obrigatórios.")
		return
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		httperr.BadRequest(c, "invalid_date", "Data em formato inválido. Use YYYY-MM-DD.")
		return
	}

	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
			return
		}
		httperr.Internal(c, "failed_to_get_barbershop", "Erro ao buscar a barbearia. Tente novamente.")
		return
	}

	var product models.BarberProduct
	if err := h.db.
		Where("id = ? AND barbershop_id = ? AND active = true", productID, shop.ID).
		First(&product).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			httperr.BadRequest(c, "product_not_found", "Serviço não encontrado para esta barbearia.")
			return
		}
		httperr.Internal(c, "failed_to_get_product", "Erro ao buscar o serviço. Tente novamente.")
		return
	}

	var barber models.User
	if err := h.db.
		Where("barbershop_id = ? AND role = ?", shop.ID, "owner").
		First(&barber).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			httperr.BadRequest(c, "barber_not_found", "Barbeiro não encontrado para esta barbearia.")
			return
		}
		httperr.Internal(c, "failed_to_get_barber", "Erro ao buscar o barbeiro. Tente novamente.")
		return
	}

	slots, err := h.generateAvailabilitySlots(barber.ID, date, &product)
	if err != nil {
		httperr.Internal(c, "failed_to_generate_slots", "Erro ao gerar horários disponíveis.")
		return
	}

	c.JSON(200, gin.H{
		"date":    dateStr,
		"product": product,
		"slots":   slots,
	})
}

func (h *PublicHandler) generateAvailabilitySlots(barberID uint, date time.Time, product *models.BarberProduct) ([]TimeSlot, error) {
	weekday := int(date.Weekday())

	var wh models.WorkingHours
	if err := h.db.
		Where("barber_id = ? AND weekday = ?", barberID, weekday).
		First(&wh).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			return []TimeSlot{}, nil
		}
		return nil, err
	}
	if !wh.Active || wh.StartTime == "" || wh.EndTime == "" {
		return []TimeSlot{}, nil
	}

	parseHMOnDate := func(hm string) (time.Time, error) {
		t, err := time.Parse("15:04", hm)
		if err != nil {
			return time.Time{}, err
		}
		return time.Date(date.Year(), date.Month(), date.Day(), t.Hour(), t.Minute(), 0, 0, time.Local), nil
	}

	dayStart, err := parseHMOnDate(wh.StartTime)
	if err != nil {
		return nil, err
	}
	dayEnd, err := parseHMOnDate(wh.EndTime)
	if err != nil {
		return nil, err
	}

	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
	endOfDay := startOfDay.Add(24 * time.Hour)

	var appointments []models.Appointment
	if err := h.db.
		Where("barber_id = ? AND status = ? AND start_time >= ? AND start_time < ?",
			barberID, "scheduled", startOfDay, endOfDay).
		Order("start_time ASC").
		Find(&appointments).Error; err != nil {
		return nil, err
	}

	slotDuration := time.Duration(product.DurationMin) * time.Minute
	var slots []TimeSlot

	for current := dayStart; current.Add(slotDuration).Before(dayEnd) || current.Add(slotDuration).Equal(dayEnd); current = current.Add(slotDuration) {
		slotStart := current
		slotEnd := current.Add(slotDuration)

		if wh.LunchStart != "" && wh.LunchEnd != "" {
			lunchStart, err := parseHMOnDate(wh.LunchStart)
			if err != nil {
				return nil, err
			}
			lunchEnd, err := parseHMOnDate(wh.LunchEnd)
			if err != nil {
				return nil, err
			}
			if slotStart.Before(lunchEnd) && slotEnd.After(lunchStart) {
				continue
			}
		}

		if slotConflicts(slotStart, slotEnd, appointments) {
			continue
		}

		slots = append(slots, TimeSlot{
			Start: slotStart.Format("15:04"),
			End:   slotEnd.Format("15:04"),
		})
	}

	return slots, nil
}

func slotConflicts(slotStart, slotEnd time.Time, appointments []models.Appointment) bool {
	for _, ap := range appointments {
		if slotStart.Before(ap.EndTime) && ap.StartTime.Before(slotEnd) {
			return true
		}
	}
	return false
}

// ---------- Agendamento público ----------
func (h *PublicHandler) CreateAppointment(c *gin.Context) {
	slug := c.Param("slug")

	var shop models.Barbershop
	if err := h.db.Where("slug = ?", slug).First(&shop).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			httperr.NotFound(c, "barbershop_not_found", "Barbearia não encontrada.")
			return
		}
		httperr.Internal(c, "failed_to_get_barbershop", "Erro ao buscar a barbearia. Tente novamente.")
		return
	}

	var req PublicCreateAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid_request", "Dados inválidos na requisição.")
		return
	}

	start, err := time.Parse("2006-01-02 15:04", req.Date+" "+req.Time)
	if err != nil {
		httperr.BadRequest(c, "invalid_date_or_time", "Data ou horário em formato inválido.")
		return
	}

	if publicIsInThePastOrTooSoon(start) {
		httperr.BadRequest(c, "too_soon_or_in_past", "Não é possível agendar para o passado ou com menos de 2 horas de antecedência.")
		return
	}

	var product models.BarberProduct
	if err := h.db.
		Where("id = ? AND barbershop_id = ? AND active = true", req.ProductID, shop.ID).
		First(&product).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			httperr.BadRequest(c, "product_not_found", "Serviço não encontrado para esta barbearia.")
			return
		}
		httperr.Internal(c, "failed_to_get_product", "Erro ao buscar o serviço. Tente novamente.")
		return
	}

	var barber models.User
	if err := h.db.
		Where("barbershop_id = ? AND role = ?", shop.ID, "owner").
		First(&barber).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			httperr.BadRequest(c, "barber_not_found", "Barbeiro não encontrado para esta barbearia.")
			return
		}
		httperr.Internal(c, "failed_to_get_barber", "Erro ao buscar o barbeiro. Tente novamente.")
		return
	}

	end := start.Add(time.Duration(product.DurationMin) * time.Minute)

	slots, err := h.generateAvailabilitySlots(barber.ID, start, &product)
	if err != nil {
		httperr.Internal(c, "failed_to_check_slots", "Erro ao validar a disponibilidade do horário.")
		return
	}

	slotOk := false
	for _, s := range slots {
		if s.Start == start.Format("15:04") && s.End == end.Format("15:04") {
			slotOk = true
			break
		}
	}
	if !slotOk {
		httperr.BadRequest(c, "slot_not_available", "Este horário não está mais disponível. Atualize a página.")
		return
	}

	var client models.Client
	if err := h.db.
		Where("barbershop_id = ? And phone = ?", shop.ID, req.ClientPhone).
		First(&client).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			client = models.Client{
				BarbershopID: shop.ID,
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
		BarbershopID:    shop.ID,
		BarberID:        barber.ID,
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

	if err := h.db.
		Preload("Client").
		Preload("BarberProduct").
		Preload("Barbershop").
		Preload("Barber").
		First(&appointment, appointment.ID).Error; err != nil {

		c.JSON(201, appointment)
		return
	}

	c.JSON(201, appointment)
}
