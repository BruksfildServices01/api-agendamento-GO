package financial

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
)

var ErrBarbershopNotFound = errors.New("barbershop not found")
var ErrInvalidPeriod = errors.New("invalid period, expected: week|month")

// Input parameters for the financial query.
type Input struct {
	BarbershopID uint
	Period       PeriodType // week|month; default = week
}

// Query is the read-only financial service.
type Query struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Query {
	return &Query{db: db}
}

// ----------------------------------------------------------------
// Execute
// ----------------------------------------------------------------

func (q *Query) Execute(ctx context.Context, input Input) (*ResponseDTO, error) {
	period := input.Period
	if period == "" {
		period = PeriodWeek
	}
	if period != PeriodWeek && period != PeriodMonth {
		return nil, ErrInvalidPeriod
	}

	// Load barbershop timezone
	var shop struct {
		Timezone string
	}
	if err := q.db.WithContext(ctx).
		Table("barbershops").
		Select("timezone").
		Where("id = ?", input.BarbershopID).
		First(&shop).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBarbershopNotFound
		}
		return nil, err
	}

	loc := timezone.Location(shop.Timezone)
	startUTC, endUTC := periodRange(period, loc)
	now := time.Now().UTC()

	dateFrom := startUTC.In(loc).Format("2006-01-02")
	dateTo := endUTC.Add(-time.Second).In(loc).Format("2006-01-02")

	realized, err := q.loadRealized(ctx, input.BarbershopID, startUTC, endUTC)
	if err != nil {
		return nil, err
	}

	expectation, err := q.loadExpectation(ctx, input.BarbershopID, now, endUTC)
	if err != nil {
		return nil, err
	}

	presumed, err := q.loadPresumed(ctx, input.BarbershopID, startUTC, now)
	if err != nil {
		return nil, err
	}

	losses, err := q.loadLosses(ctx, input.BarbershopID, startUTC, endUTC)
	if err != nil {
		return nil, err
	}

	return &ResponseDTO{
		Period:      string(period),
		DateFrom:    dateFrom,
		DateTo:      dateTo,
		Timezone:    shop.Timezone,
		Realized:    realized,
		Expectation: expectation,
		Presumed:    presumed,
		Losses:      losses,
	}, nil
}

// ----------------------------------------------------------------
// Realized — fechamentos + pedidos pagos no período
// ----------------------------------------------------------------

func (q *Query) loadRealized(ctx context.Context, barbershopID uint, start, end time.Time) (RealizedDTO, error) {
	// Service revenue from closures
	var closureResult struct {
		ServicesCents      int64 `gorm:"column:services_cents"`
		SubscriptionsCents int64 `gorm:"column:subscriptions_cents"`
		Count              int   `gorm:"column:count"`
	}
	err := q.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(SUM(COALESCE(ac.final_amount_cents, ac.reference_amount_cents)), 0) AS services_cents,
			COALESCE(SUM(
				CASE WHEN ac.subscription_covered
				     THEN COALESCE(ac.final_amount_cents, ac.reference_amount_cents)
				     ELSE 0 END
			), 0) AS subscriptions_cents,
			COUNT(*) AS count
		FROM appointment_closures ac
		JOIN appointments a ON a.id = ac.appointment_id
		WHERE ac.barbershop_id = ?
		  AND a.start_time >= ?
		  AND a.start_time < ?
	`, barbershopID, start, end).Scan(&closureResult).Error
	if err != nil {
		return RealizedDTO{}, err
	}

	// Product revenue from paid orders
	var orderResult struct {
		ProductsCents int64 `gorm:"column:products_cents"`
		Count         int   `gorm:"column:count"`
	}
	err = q.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(SUM(o.total_amount), 0) AS products_cents,
			COUNT(*) AS count
		FROM orders o
		WHERE o.barbershop_id = ?
		  AND o.status = 'paid'
		  AND o.created_at >= ?
		  AND o.created_at < ?
	`, barbershopID, start, end).Scan(&orderResult).Error
	if err != nil {
		return RealizedDTO{}, err
	}

	serviceNet := closureResult.ServicesCents - closureResult.SubscriptionsCents

	return RealizedDTO{
		TotalCents:         closureResult.ServicesCents + orderResult.ProductsCents,
		ServicesCents:      serviceNet,
		ProductsCents:      orderResult.ProductsCents,
		SubscriptionsCents: closureResult.SubscriptionsCents,
		ClosuresCount:      closureResult.Count,
		PaidOrdersCount:    orderResult.Count,
	}, nil
}

// ----------------------------------------------------------------
// Expectation — agenda futura dentro do período
// ----------------------------------------------------------------

func (q *Query) loadExpectation(ctx context.Context, barbershopID uint, from, end time.Time) (ExpectationDTO, error) {
	// Future appointments: service price sum
	var apptResult struct {
		ServicesCents int64 `gorm:"column:services_cents"`
		Count         int   `gorm:"column:count"`
	}
	err := q.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(SUM(bs.price), 0) AS services_cents,
			COUNT(a.id) AS count
		FROM appointments a
		JOIN barbershop_services bs ON bs.id = a.barber_product_id
		WHERE a.barbershop_id = ?
		  AND a.status IN ('scheduled', 'awaiting_payment')
		  AND a.start_time >= ?
		  AND a.start_time < ?
	`, barbershopID, from, end).Scan(&apptResult).Error
	if err != nil {
		return ExpectationDTO{}, err
	}

	// Suggestions for those future appointments
	var suggResult struct {
		SuggestionsCents int64 `gorm:"column:suggestions_cents"`
	}
	err = q.db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(prod.price), 0) AS suggestions_cents
		FROM appointments a
		JOIN service_suggested_products ssp ON ssp.service_id = a.barber_product_id
			AND ssp.barbershop_id = a.barbershop_id
			AND ssp.active = true
		JOIN products prod ON prod.id = ssp.product_id
		WHERE a.barbershop_id = ?
		  AND a.status IN ('scheduled', 'awaiting_payment')
		  AND a.start_time >= ?
		  AND a.start_time < ?
	`, barbershopID, from, end).Scan(&suggResult).Error
	if err != nil {
		return ExpectationDTO{}, err
	}

	return ExpectationDTO{
		TotalCents:        apptResult.ServicesCents + suggResult.SuggestionsCents,
		ServicesCents:     apptResult.ServicesCents,
		SuggestionsCents:  suggResult.SuggestionsCents,
		AppointmentsCount: apptResult.Count,
	}, nil
}

// ----------------------------------------------------------------
// Presumed — agendamentos passados sem fechamento
// ----------------------------------------------------------------

func (q *Query) loadPresumed(ctx context.Context, barbershopID uint, start, now time.Time) (PresumedDTO, error) {
	var result struct {
		TotalCents int64 `gorm:"column:total_cents"`
		Count      int   `gorm:"column:count"`
	}
	err := q.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(SUM(bs.price), 0) AS total_cents,
			COUNT(a.id) AS count
		FROM appointments a
		JOIN barbershop_services bs ON bs.id = a.barber_product_id
		LEFT JOIN appointment_closures ac ON ac.appointment_id = a.id
		WHERE a.barbershop_id = ?
		  AND a.status = 'scheduled'
		  AND a.start_time >= ?
		  AND a.start_time < ?
		  AND ac.id IS NULL
	`, barbershopID, start, now).Scan(&result).Error
	if err != nil {
		return PresumedDTO{}, err
	}

	return PresumedDTO{
		TotalCents:        result.TotalCents,
		AppointmentsCount: result.Count,
	}, nil
}

// ----------------------------------------------------------------
// Losses — no-show, cancelamentos, sugestões não vendidas
// ----------------------------------------------------------------

func (q *Query) loadLosses(ctx context.Context, barbershopID uint, start, end time.Time) (LossesDTO, error) {
	type lossRow struct {
		LossType    string `gorm:"column:loss_type"`
		AmountCents int64  `gorm:"column:amount_cents"`
		Count       int    `gorm:"column:count"`
	}

	// No-show losses: service price of no-show appointments
	var noShowResult lossRow
	err := q.db.WithContext(ctx).Raw(`
		SELECT
			'no_show' AS loss_type,
			COALESCE(SUM(bs.price), 0) AS amount_cents,
			COUNT(a.id) AS count
		FROM appointments a
		JOIN barbershop_services bs ON bs.id = a.barber_product_id
		WHERE a.barbershop_id = ?
		  AND a.status = 'no_show'
		  AND a.start_time >= ?
		  AND a.start_time < ?
	`, barbershopID, start, end).Scan(&noShowResult).Error
	if err != nil {
		return LossesDTO{}, err
	}
	noShowResult.LossType = "no_show"

	// Cancellation losses
	var cancelResult lossRow
	err = q.db.WithContext(ctx).Raw(`
		SELECT
			'cancellation' AS loss_type,
			COALESCE(SUM(bs.price), 0) AS amount_cents,
			COUNT(a.id) AS count
		FROM appointments a
		JOIN barbershop_services bs ON bs.id = a.barber_product_id
		WHERE a.barbershop_id = ?
		  AND a.status = 'cancelled'
		  AND a.start_time >= ?
		  AND a.start_time < ?
	`, barbershopID, start, end).Scan(&cancelResult).Error
	if err != nil {
		return LossesDTO{}, err
	}
	cancelResult.LossType = "cancellation"

	// Suggestion not sold: completed closures where suggestion_removed = true
	var suggNotSoldResult lossRow
	err = q.db.WithContext(ctx).Raw(`
		SELECT
			'suggestion_not_sold' AS loss_type,
			COALESCE(SUM(prod.price), 0) AS amount_cents,
			COUNT(ac.id) AS count
		FROM appointment_closures ac
		JOIN appointments a ON a.id = ac.appointment_id
		JOIN service_suggested_products ssp
			ON ssp.service_id = ac.service_id
			AND ssp.barbershop_id = ac.barbershop_id
			AND ssp.active = true
		JOIN products prod ON prod.id = ssp.product_id
		WHERE ac.barbershop_id = ?
		  AND ac.suggestion_removed = true
		  AND a.start_time >= ?
		  AND a.start_time < ?
	`, barbershopID, start, end).Scan(&suggNotSoldResult).Error
	if err != nil {
		return LossesDTO{}, err
	}
	suggNotSoldResult.LossType = "suggestion_not_sold"

	breakdown := make([]LossItemDTO, 0, 3)
	total := int64(0)

	for _, r := range []lossRow{noShowResult, cancelResult, suggNotSoldResult} {
		if r.Count > 0 || r.AmountCents > 0 {
			breakdown = append(breakdown, LossItemDTO{
				Type:        r.LossType,
				AmountCents: r.AmountCents,
				Count:       r.Count,
			})
		}
		total += r.AmountCents
	}

	return LossesDTO{
		TotalCents: total,
		Breakdown:  breakdown,
	}, nil
}

// ----------------------------------------------------------------
// Period helpers
// ----------------------------------------------------------------

func periodRange(period PeriodType, loc *time.Location) (startUTC, endUTC time.Time) {
	now := time.Now().In(loc)
	var localStart time.Time

	switch period {
	case PeriodWeek:
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		localStart = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, loc)
	case PeriodMonth:
		localStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	}

	var localEnd time.Time
	switch period {
	case PeriodWeek:
		localEnd = localStart.AddDate(0, 0, 7)
	case PeriodMonth:
		localEnd = localStart.AddDate(0, 1, 0)
	}

	return localStart.UTC(), localEnd.UTC()
}
