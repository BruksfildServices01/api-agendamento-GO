package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
	ucPayment "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

type PaymentHandler struct {
	db           *gorm.DB
	listPayments *ucPayment.ListPaymentsForBarbershop
}

func NewPaymentHandler(
	db *gorm.DB,
	listPayments *ucPayment.ListPaymentsForBarbershop,
) *PaymentHandler {
	return &PaymentHandler{
		db:           db,
		listPayments: listPayments,
	}
}

// ======================================================
// GET /api/me/payments
// Query params:
// - status=paid|pending|expired
// - start_date=YYYY-MM-DD (inclusive, shop timezone)
// - end_date=YYYY-MM-DD   (inclusive, shop timezone)  -> converted to exclusive upper bound
// ======================================================
func (h *PaymentHandler) List(c *gin.Context) {

	// --------------------------------------------------
	// 1️⃣ Contexto (tenant)
	// --------------------------------------------------
	raw, exists := c.Get(middleware.ContextBarbershopID)
	if !exists {
		httperr.Unauthorized(c, "unauthorized", "Acesso não autorizado.")
		return
	}

	barbershopID, ok := raw.(uint)
	if !ok || barbershopID == 0 {
		httperr.Unauthorized(c, "unauthorized", "Acesso não autorizado.")
		return
	}

	// --------------------------------------------------
	// 2️⃣ Carregar barbearia (timezone)
	// --------------------------------------------------
	var shop models.Barbershop
	if err := h.db.WithContext(c.Request.Context()).
		Select("id, timezone").
		First(&shop, barbershopID).
		Error; err != nil {

		// Se não achar, trata como context inválido
		httperr.BadRequest(c, "invalid_barbershop", "Barbearia inválida.")
		return
	}

	loc := timezone.Location(shop.Timezone)

	// --------------------------------------------------
	// 3️⃣ Query params
	// --------------------------------------------------
	var (
		status    *string
		startDate *time.Time // UTC lower bound inclusive
		endDate   *time.Time // UTC upper bound exclusive
	)

	if v := c.Query("status"); v != "" {
		status = &v
	}

	// start_date (inclusive)
	if v := c.Query("start_date"); v != "" {
		tUTC, err := parseDateAsStartOfDayUTC(v, loc)
		if err != nil {
			httperr.BadRequest(
				c,
				"invalid_start_date",
				"Formato inválido para start_date (YYYY-MM-DD).",
			)
			return
		}
		startDate = tUTC
	}

	// end_date (inclusive input -> exclusive bound day+1)
	if v := c.Query("end_date"); v != "" {
		tUTC, err := parseDateAsEndExclusiveUTC(v, loc)
		if err != nil {
			httperr.BadRequest(
				c,
				"invalid_end_date",
				"Formato inválido para end_date (YYYY-MM-DD).",
			)
			return
		}
		endDate = tUTC
	}

	// --------------------------------------------------
	// 4️⃣ Use case
	// --------------------------------------------------
	payments, err := h.listPayments.Execute(
		c.Request.Context(),
		ucPayment.ListPaymentsInput{
			BarbershopID: barbershopID,
			Status:       status,
			StartDate:    startDate,
			EndDate:      endDate,
		},
	)
	if err != nil {
		httperr.Internal(c, "list_payments_failed", "Erro ao listar pagamentos.")
		return
	}

	// --------------------------------------------------
	// 5️⃣ Response
	// --------------------------------------------------
	c.JSON(http.StatusOK, gin.H{
		"data":  payments,
		"total": len(payments),
	})
}

// ======================================================
// GET /api/me/payments/cash-due
// Returns scheduled appointments with no paid digital payment —
// these are cash/in-person collections the barber still needs to receive.
// Query params: start_date, end_date (YYYY-MM-DD, barbershop timezone)
// ======================================================
func (h *PaymentHandler) CashDue(c *gin.Context) {
	raw, exists := c.Get(middleware.ContextBarbershopID)
	if !exists {
		httperr.Unauthorized(c, "unauthorized", "Acesso não autorizado.")
		return
	}
	barbershopID, ok := raw.(uint)
	if !ok || barbershopID == 0 {
		httperr.Unauthorized(c, "unauthorized", "Acesso não autorizado.")
		return
	}

	var shop models.Barbershop
	if err := h.db.WithContext(c.Request.Context()).
		Select("id, timezone").
		First(&shop, barbershopID).Error; err != nil {
		httperr.BadRequest(c, "invalid_barbershop", "Barbearia inválida.")
		return
	}
	loc := timezone.Location(shop.Timezone)

	var startDate, endDate *time.Time
	if v := c.Query("start_date"); v != "" {
		t, err := parseDateAsStartOfDayUTC(v, loc)
		if err != nil {
			httperr.BadRequest(c, "invalid_start_date", "Formato inválido para start_date (YYYY-MM-DD).")
			return
		}
		startDate = t
	}
	if v := c.Query("end_date"); v != "" {
		t, err := parseDateAsEndExclusiveUTC(v, loc)
		if err != nil {
			httperr.BadRequest(c, "invalid_end_date", "Formato inválido para end_date (YYYY-MM-DD).")
			return
		}
		endDate = t
	}

	type cashDueRow struct {
		AppointmentID uint      `gorm:"column:appointment_id"`
		StartTime     time.Time `gorm:"column:start_time"`
		ClientName    string    `gorm:"column:client_name"`
		AmountCents   int64     `gorm:"column:amount_cents"`
		ServiceName   string    `gorm:"column:service_name"`
	}

	query := `
		SELECT
			a.id AS appointment_id,
			a.start_time,
			COALESCE(c.name, 'Cliente não identificado') AS client_name,
			COALESCE(bs.price, 0) AS amount_cents,
			COALESCE(bs.name, 'Serviço') AS service_name
		FROM appointments a
		LEFT JOIN clients c ON c.id = a.client_id
		LEFT JOIN barbershop_services bs ON bs.id = a.barber_product_id
		WHERE a.barbershop_id = ?
		  AND a.status = 'scheduled'
		  AND NOT EXISTS (
		    SELECT 1 FROM payments p
		    WHERE p.appointment_id = a.id AND p.status = 'paid'
		  )`

	args := []any{barbershopID}
	if startDate != nil {
		query += " AND a.start_time >= ?"
		args = append(args, *startDate)
	}
	if endDate != nil {
		query += " AND a.start_time < ?"
		args = append(args, *endDate)
	}
	query += " ORDER BY a.start_time ASC"

	var rows []cashDueRow
	if err := h.db.WithContext(c.Request.Context()).Raw(query, args...).Scan(&rows).Error; err != nil {
		httperr.Internal(c, "cash_due_failed", "Erro ao buscar atendimentos a receber.")
		return
	}

	type cashDueItem struct {
		AppointmentID uint      `json:"appointment_id"`
		StartTime     time.Time `json:"start_time"`
		ClientName    string    `json:"client_name"`
		AmountCents   int64     `json:"amount_cents"`
		ServiceName   string    `json:"service_name"`
	}

	items := make([]cashDueItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, cashDueItem{
			AppointmentID: r.AppointmentID,
			StartTime:     r.StartTime,
			ClientName:    r.ClientName,
			AmountCents:   r.AmountCents,
			ServiceName:   r.ServiceName,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": items, "total": len(items)})
}

// --------------------------------------------------
// Helpers (timezone-safe)
// --------------------------------------------------

// parseDateAsStartOfDayUTC interprets YYYY-MM-DD as 00:00 at barbershop timezone, converted to UTC.
func parseDateAsStartOfDayUTC(value string, loc *time.Location) (*time.Time, error) {
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}

	local := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	utc := local.UTC()
	return &utc, nil
}

func parseDateAsEndExclusiveUTC(value string, loc *time.Location) (*time.Time, error) {
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}

	localNextDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc).Add(24 * time.Hour)
	utc := localNextDay.UTC()
	return &utc, nil
}
