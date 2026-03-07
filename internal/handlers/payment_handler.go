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
