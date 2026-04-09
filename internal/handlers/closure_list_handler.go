package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
)

// ClosureListHandler serves paginated appointment closures for the barbershop owner.
type ClosureListHandler struct {
	db *gorm.DB
}

func NewClosureListHandler(db *gorm.DB) *ClosureListHandler {
	return &ClosureListHandler{db: db}
}

type ClosureListItemResponse struct {
	ID            uint      `json:"id"`
	ServiceName   string    `json:"service_name"`
	ClientName    string    `json:"client_name,omitempty"`
	AmountCents   int64     `json:"amount_cents"`
	PaymentMethod string    `json:"payment_method"`
	Subscription  bool      `json:"subscription_covered"`
	AppointmentID uint      `json:"appointment_id"`
	CreatedAt     time.Time `json:"created_at"`
}

type ClosureDetailResponse struct {
	ID            uint      `json:"id"`
	AppointmentID uint      `json:"appointment_id"`
	ServiceName   string    `json:"service_name"`
	AmountCents   int64     `json:"amount_cents"`
	PaymentMethod string    `json:"payment_method"`
	Subscription  bool      `json:"subscription_covered"`
	OperationalNote string  `json:"operational_note,omitempty"`

	Client *RichOrderClientInfo `json:"client,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// List handles GET /api/me/closures?page=&limit=
func (h *ClosureListHandler) List(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		httperr.Unauthorized(c, "invalid_barbershop", "Barbershop inválida.")
		return
	}

	page, _ := parsePositiveIntDefault(c.Query("page"), 1)
	limit, _ := parsePositiveIntDefault(c.Query("limit"), 20)
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	type row struct {
		ID            uint      `gorm:"column:id"`
		ServiceName   string    `gorm:"column:service_name"`
		ClientName    string    `gorm:"column:client_name"`
		AmountCents   int64     `gorm:"column:amount_cents"`
		PaymentMethod string    `gorm:"column:payment_method"`
		Subscription  bool      `gorm:"column:subscription_covered"`
		AppointmentID uint      `gorm:"column:appointment_id"`
		CreatedAt     time.Time `gorm:"column:created_at"`
	}

	var total int64
	if err := h.db.WithContext(c.Request.Context()).
		Table("appointment_closures ac").
		Joins("JOIN appointments a ON a.id = ac.appointment_id").
		Where("ac.barbershop_id = ?", barbershopID).
		Count(&total).Error; err != nil {
		httperr.Internal(c, "failed_to_count_closures", "Falha ao contar atendimentos.")
		return
	}

	var rows []row
	err := h.db.WithContext(c.Request.Context()).Raw(`
		SELECT
			ac.id,
			COALESCE(NULLIF(ac.actual_service_name, ''), NULLIF(ac.service_name, ''), 'Serviço removido') AS service_name,
			COALESCE(c.name, '') AS client_name,
			COALESCE(ac.final_amount_cents, ac.reference_amount_cents) AS amount_cents,
			COALESCE(ac.payment_method, '') AS payment_method,
			ac.subscription_covered,
			ac.appointment_id,
			a.start_time AS created_at
		FROM appointment_closures ac
		JOIN appointments a ON a.id = ac.appointment_id
		LEFT JOIN clients c ON c.id = a.client_id
		WHERE ac.barbershop_id = ?
		ORDER BY a.start_time DESC, ac.id DESC
		LIMIT ? OFFSET ?
	`, barbershopID, limit, offset).Scan(&rows).Error
	if err != nil {
		httperr.Internal(c, "failed_to_list_closures", "Falha ao listar atendimentos.")
		return
	}

	items := make([]ClosureListItemResponse, 0, len(rows))
	for _, r := range rows {
		items = append(items, ClosureListItemResponse{
			ID:            r.ID,
			ServiceName:   r.ServiceName,
			ClientName:    r.ClientName,
			AmountCents:   r.AmountCents,
			PaymentMethod: r.PaymentMethod,
			Subscription:  r.Subscription,
			AppointmentID: r.AppointmentID,
			CreatedAt:     r.CreatedAt,
		})
	}

	totalPages := 0
	if total > 0 {
		totalPages = (int(total) + limit - 1) / limit
	}

	c.JSON(http.StatusOK, gin.H{
		"data":        items,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": totalPages,
	})
}

// GetByID handles GET /api/me/closures/:id
func (h *ClosureListHandler) GetByID(c *gin.Context) {
	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		httperr.Unauthorized(c, "invalid_barbershop", "Barbershop inválida.")
		return
	}

	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id64 == 0 {
		httperr.BadRequest(c, "invalid_id", "ID inválido.")
		return
	}

	type detailRow struct {
		ID              uint      `gorm:"column:id"`
		AppointmentID   uint      `gorm:"column:appointment_id"`
		ServiceName     string    `gorm:"column:service_name"`
		AmountCents     int64     `gorm:"column:amount_cents"`
		PaymentMethod   string    `gorm:"column:payment_method"`
		Subscription    bool      `gorm:"column:subscription_covered"`
		OperationalNote string    `gorm:"column:operational_note"`
		ClientID        *uint     `gorm:"column:client_id"`
		ClientName      string    `gorm:"column:client_name"`
		ClientPhone     string    `gorm:"column:client_phone"`
		ClientEmail     string    `gorm:"column:client_email"`
		CreatedAt       time.Time `gorm:"column:created_at"`
	}

	var r detailRow
	err = h.db.WithContext(c.Request.Context()).Raw(`
		SELECT
			ac.id,
			ac.appointment_id,
			COALESCE(NULLIF(ac.actual_service_name, ''), NULLIF(ac.service_name, ''), 'Serviço removido') AS service_name,
			COALESCE(ac.final_amount_cents, ac.reference_amount_cents) AS amount_cents,
			COALESCE(ac.payment_method, '') AS payment_method,
			ac.subscription_covered,
			COALESCE(ac.operational_note, '') AS operational_note,
			a.client_id,
			COALESCE(c.name,  '') AS client_name,
			COALESCE(c.phone, '') AS client_phone,
			COALESCE(c.email, '') AS client_email,
			a.start_time AS created_at
		FROM appointment_closures ac
		JOIN appointments a ON a.id = ac.appointment_id
		LEFT JOIN clients c ON c.id = a.client_id
		WHERE ac.id = ? AND ac.barbershop_id = ?
		LIMIT 1
	`, id64, barbershopID).Scan(&r).Error
	if err != nil {
		httperr.Internal(c, "failed_to_get_closure", "Falha ao buscar atendimento.")
		return
	}
	if r.ID == 0 {
		httperr.NotFound(c, "closure_not_found", "Atendimento não encontrado.")
		return
	}

	resp := ClosureDetailResponse{
		ID:              r.ID,
		AppointmentID:   r.AppointmentID,
		ServiceName:     r.ServiceName,
		AmountCents:     r.AmountCents,
		PaymentMethod:   r.PaymentMethod,
		Subscription:    r.Subscription,
		OperationalNote: r.OperationalNote,
		CreatedAt:       r.CreatedAt,
	}

	if r.ClientID != nil && r.ClientName != "" {
		resp.Client = &RichOrderClientInfo{
			ID:    *r.ClientID,
			Name:  r.ClientName,
			Phone: r.ClientPhone,
			Email: r.ClientEmail,
		}
	}

	c.JSON(http.StatusOK, resp)
}
