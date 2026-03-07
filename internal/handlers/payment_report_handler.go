package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	domainAppointment "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/middleware"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
	uc "github.com/BruksfildServices01/barber-scheduler/internal/usecase/payment"
)

type PaymentReportHandler struct {
	uc   *uc.GetPaymentSummary
	repo domainAppointment.Repository
}

func NewPaymentReportHandler(
	uc *uc.GetPaymentSummary,
	repo domainAppointment.Repository,
) *PaymentReportHandler {
	return &PaymentReportHandler{
		uc:   uc,
		repo: repo,
	}
}

func (h *PaymentReportHandler) Summary(c *gin.Context) {

	barbershopID := c.GetUint(middleware.ContextBarbershopID)
	if barbershopID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid_barbershop",
		})
		return
	}

	// --------------------------------------------------
	// 1️⃣ Carrega barbearia (multi-tenant timezone)
	// --------------------------------------------------
	shop, err := h.repo.GetBarbershopByID(c.Request.Context(), barbershopID)
	if err != nil || shop == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid_barbershop",
		})
		return
	}

	loc := timezone.Location(shop.Timezone)

	var from *time.Time
	var to *time.Time

	// --------------------------------------------------
	// 2️⃣ Parse seguro com timezone
	//   - from: inclusive
	//   - to:   exclusive (day+1 when YYYY-MM-DD)
	// --------------------------------------------------

	if v := c.Query("from"); v != "" {
		t, err := parseFromWithTimezone(v, loc)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid_from_format",
			})
			return
		}
		from = t
	}

	if v := c.Query("to"); v != "" {
		t, err := parseToExclusiveWithTimezone(v, loc)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid_to_format",
			})
			return
		}
		to = t
	}

	// --------------------------------------------------
	// 3️⃣ Executa use case
	// --------------------------------------------------

	summary, err := h.uc.Execute(
		c.Request.Context(),
		barbershopID,
		from,
		to,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed_to_generate_report",
		})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// --------------------------------------------------
// Helpers: parsing timezone-safe
// --------------------------------------------------

// parseFromWithTimezone:
// - RFC3339 => UTC instant
// - YYYY-MM-DD => start of day in shop tz => UTC (inclusive)
func parseFromWithTimezone(value string, loc *time.Location) (*time.Time, error) {

	// 1️⃣ RFC3339 completo (com offset ou Z)
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		utc := t.UTC()
		return &utc, nil
	}

	// 2️⃣ Apenas data (YYYY-MM-DD) -> start of day
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, fmt.Errorf("invalid date format")
	}

	localStart := time.Date(
		t.Year(),
		t.Month(),
		t.Day(),
		0, 0, 0, 0,
		loc,
	)

	utc := localStart.UTC()
	return &utc, nil
}

// parseToExclusiveWithTimezone:
// - RFC3339 => UTC instant
// - YYYY-MM-DD => next day start in shop tz => UTC (exclusive upper bound)
func parseToExclusiveWithTimezone(value string, loc *time.Location) (*time.Time, error) {

	// 1️⃣ RFC3339 completo (com offset ou Z)
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		utc := t.UTC()
		return &utc, nil
	}

	// 2️⃣ Apenas data (YYYY-MM-DD) -> next day start
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, fmt.Errorf("invalid date format")
	}

	localNextDay := time.Date(
		t.Year(),
		t.Month(),
		t.Day(),
		0, 0, 0, 0,
		loc,
	).Add(24 * time.Hour)

	utc := localNextDay.UTC()
	return &utc, nil
}
