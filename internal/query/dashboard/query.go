package dashboard

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
)

var ErrBarbershopNotFound = errors.New("barbershop not found")
var ErrInvalidPeriod = errors.New("invalid period, expected: day|week|month")

// Input parameters for the dashboard query.
type Input struct {
	BarbershopID uint
	Period       PeriodType // day|week|month; default = week
}

// Query is the read-only service for the dashboard.
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
	// 1. Validate period
	period := input.Period
	if period == "" {
		period = PeriodWeek
	}
	if period != PeriodDay && period != PeriodWeek && period != PeriodMonth {
		return nil, ErrInvalidPeriod
	}

	// 2. Load barbershop timezone
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

	dateFrom := startUTC.In(loc).Format("2006-01-02")
	dateTo := endUTC.Add(-time.Second).In(loc).Format("2006-01-02")

	// 3. Run all queries in parallel (independent)
	production, err := q.loadProduction(ctx, input.BarbershopID, startUTC, endUTC)
	if err != nil {
		return nil, err
	}

	revenue, err := q.loadRevenue(ctx, input.BarbershopID, startUTC, endUTC)
	if err != nil {
		return nil, err
	}

	clients, err := q.loadClients(ctx, input.BarbershopID, startUTC, endUTC)
	if err != nil {
		return nil, err
	}

	topServices, err := q.loadTopServices(ctx, input.BarbershopID, startUTC, endUTC)
	if err != nil {
		return nil, err
	}

	topProducts, err := q.loadTopProducts(ctx, input.BarbershopID, startUTC, endUTC)
	if err != nil {
		return nil, err
	}

	return &ResponseDTO{
		Period:      string(period),
		DateFrom:    dateFrom,
		DateTo:      dateTo,
		Timezone:    shop.Timezone,
		Production:  production,
		Revenue:     revenue,
		Clients:     clients,
		TopServices: topServices,
		TopProducts: topProducts,
	}, nil
}

// ----------------------------------------------------------------
// Production
// ----------------------------------------------------------------

func (q *Query) loadProduction(ctx context.Context, barbershopID uint, start, end time.Time) (ProductionDTO, error) {
	type row struct {
		Status string `gorm:"column:status"`
		Count  int    `gorm:"column:count"`
	}

	var rows []row
	err := q.db.WithContext(ctx).Raw(`
		SELECT status, COUNT(*) AS count
		FROM appointments
		WHERE barbershop_id = ?
		  AND start_time >= ?
		  AND start_time < ?
		GROUP BY status
	`, barbershopID, start, end).Scan(&rows).Error
	if err != nil {
		return ProductionDTO{}, err
	}

	var p ProductionDTO
	for _, r := range rows {
		p.Total += r.Count
		switch r.Status {
		case "completed":
			p.Completed = r.Count
		case "cancelled":
			p.Cancelled = r.Count
		case "no_show":
			p.NoShow = r.Count
		case "scheduled", "awaiting_payment":
			p.Scheduled += r.Count
		}
	}

	denominator := p.Completed + p.Cancelled + p.NoShow
	if denominator > 0 {
		p.AttendanceRate = float64(p.Completed) / float64(denominator)
	}

	// Suggestion conversion: closures where the service had an active suggestion
	var suggRow struct {
		Total   int `gorm:"column:total"`
		Kept    int `gorm:"column:kept"`
		Removed int `gorm:"column:removed"`
	}
	err = q.db.WithContext(ctx).Raw(`
		SELECT
			COUNT(*) AS total,
			SUM(CASE WHEN ac.suggestion_removed = false THEN 1 ELSE 0 END) AS kept,
			SUM(CASE WHEN ac.suggestion_removed = true  THEN 1 ELSE 0 END) AS removed
		FROM appointment_closures ac
		JOIN appointments a ON a.id = ac.appointment_id
		JOIN service_suggested_products ssp
			ON ssp.service_id = ac.service_id
			AND ssp.barbershop_id = ac.barbershop_id
			AND ssp.active = true
		WHERE ac.barbershop_id = ?
		  AND a.start_time >= ?
		  AND a.start_time < ?
	`, barbershopID, start, end).Scan(&suggRow).Error
	if err != nil {
		return ProductionDTO{}, err
	}

	p.SuggestionTotal = suggRow.Total
	p.SuggestionKept = suggRow.Kept
	p.SuggestionRemoved = suggRow.Removed

	return p, nil
}

// ----------------------------------------------------------------
// Revenue
// ----------------------------------------------------------------

func (q *Query) loadRevenue(ctx context.Context, barbershopID uint, start, end time.Time) (RevenueDTO, error) {
	// Service revenue from closures.
	var serviceRevenue struct {
		Total            int64 `gorm:"column:total"`
		SubscriptionPart int64 `gorm:"column:subscription_part"`
		ClosuresCount    int64 `gorm:"column:closures_count"`
	}
	err := q.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(SUM(COALESCE(ac.final_amount_cents, ac.reference_amount_cents)), 0) AS total,
			COALESCE(SUM(CASE WHEN ac.subscription_covered THEN COALESCE(ac.final_amount_cents, ac.reference_amount_cents) ELSE 0 END), 0) AS subscription_part,
			COUNT(*) AS closures_count
		FROM appointment_closures ac
		JOIN appointments a ON a.id = ac.appointment_id
		WHERE ac.barbershop_id = ?
		  AND a.start_time >= ?
		  AND a.start_time < ?
	`, barbershopID, start, end).Scan(&serviceRevenue).Error
	if err != nil {
		return RevenueDTO{}, err
	}

	// Product revenue — includes closure-linked orders (may be pending in legacy data).
	var productRevenue struct {
		Total int64 `gorm:"column:total"`
	}
	err = q.db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(o.total_amount), 0) AS total
		FROM orders o
		WHERE o.barbershop_id = ?
		  AND o.created_at >= ?
		  AND o.created_at < ?
		  AND (
		      o.status = 'paid'
		      OR EXISTS (
		          SELECT 1 FROM appointment_closures ac
		          WHERE ac.additional_order_id = o.id
		      )
		  )
	`, barbershopID, start, end).Scan(&productRevenue).Error
	if err != nil {
		return RevenueDTO{}, err
	}

	// Suggestion-linked product revenue.
	var suggestionRevenue struct {
		Total int64 `gorm:"column:total"`
	}
	err = q.db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(o.total_amount), 0) AS total
		FROM orders o
		WHERE o.barbershop_id = ?
		  AND o.created_at >= ?
		  AND o.created_at < ?
		  AND EXISTS (
		      SELECT 1 FROM appointment_closures ac
		      WHERE ac.additional_order_id = o.id
		  )
	`, barbershopID, start, end).Scan(&suggestionRevenue).Error
	if err != nil {
		return RevenueDTO{}, err
	}

	servicesCents := serviceRevenue.Total - serviceRevenue.SubscriptionPart

	var avgTicket int64
	if serviceRevenue.ClosuresCount > 0 {
		avgTicket = (serviceRevenue.Total + productRevenue.Total) / serviceRevenue.ClosuresCount
	}

	return RevenueDTO{
		TotalCents:              serviceRevenue.Total + productRevenue.Total,
		ServicesCents:           servicesCents,
		ProductsCents:           productRevenue.Total,
		ProductsSuggestionCents: suggestionRevenue.Total,
		ProductsStandaloneCents: productRevenue.Total - suggestionRevenue.Total,
		SubscriptionsCents:      serviceRevenue.SubscriptionPart,
		AvgTicketCents:          avgTicket,
	}, nil
}

// ----------------------------------------------------------------
// Clients
// ----------------------------------------------------------------

func (q *Query) loadClients(ctx context.Context, barbershopID uint, start, end time.Time) (ClientsDTO, error) {
	type row struct {
		ClientID      uint      `gorm:"column:client_id"`
		FirstEverAppt time.Time `gorm:"column:first_ever_appt"`
	}

	var rows []row
	err := q.db.WithContext(ctx).Raw(`
		SELECT
			a.client_id,
			MIN(all_a.start_time) AS first_ever_appt
		FROM appointments a
		JOIN appointments all_a
			ON all_a.client_id = a.client_id
			AND all_a.barbershop_id = a.barbershop_id
		WHERE a.barbershop_id = ?
		  AND a.client_id IS NOT NULL
		  AND a.start_time >= ?
		  AND a.start_time < ?
		GROUP BY a.client_id
	`, barbershopID, start, end).Scan(&rows).Error
	if err != nil {
		return ClientsDTO{}, err
	}

	var clients ClientsDTO
	clients.Total = len(rows)
	for _, r := range rows {
		if !r.FirstEverAppt.Before(start) {
			clients.New++
		} else {
			clients.Returning++
		}
	}

	// Active subscribers
	var activeSubscribers int
	err = q.db.WithContext(ctx).Raw(`
		SELECT COUNT(DISTINCT client_id)
		FROM subscriptions
		WHERE barbershop_id = ?
		  AND status = 'active'
	`, barbershopID).Scan(&activeSubscribers).Error
	if err != nil {
		return ClientsDTO{}, err
	}
	clients.WithActiveSubscription = activeSubscribers

	return clients, nil
}

// ----------------------------------------------------------------
// Top Services
// ----------------------------------------------------------------

func (q *Query) loadTopServices(ctx context.Context, barbershopID uint, start, end time.Time) ([]ServiceRankItem, error) {
	type row struct {
		ServiceID    uint   `gorm:"column:service_id"`
		ServiceName  string `gorm:"column:service_name"`
		Count        int    `gorm:"column:count"`
		RevenueCents int64  `gorm:"column:revenue_cents"`
	}

	var rows []row
	err := q.db.WithContext(ctx).Raw(`
		SELECT
			ac.service_id,
			ac.service_name,
			COUNT(*)  AS count,
			COALESCE(SUM(COALESCE(ac.final_amount_cents, ac.reference_amount_cents)), 0) AS revenue_cents
		FROM appointment_closures ac
		JOIN appointments a ON a.id = ac.appointment_id
		WHERE ac.barbershop_id = ?
		  AND ac.service_id IS NOT NULL
		  AND a.start_time >= ?
		  AND a.start_time < ?
		GROUP BY ac.service_id, ac.service_name
		ORDER BY revenue_cents DESC
		LIMIT 5
	`, barbershopID, start, end).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]ServiceRankItem, len(rows))
	for i, r := range rows {
		result[i] = ServiceRankItem{
			ServiceID:    r.ServiceID,
			ServiceName:  r.ServiceName,
			Count:        r.Count,
			RevenueCents: r.RevenueCents,
		}
	}

	return result, nil
}

// ----------------------------------------------------------------
// Top Products
// ----------------------------------------------------------------

func (q *Query) loadTopProducts(ctx context.Context, barbershopID uint, start, end time.Time) ([]ProductRankItem, error) {
	type row struct {
		ProductID    uint   `gorm:"column:product_id"`
		ProductName  string `gorm:"column:product_name"`
		Quantity     int    `gorm:"column:quantity"`
		RevenueCents int64  `gorm:"column:revenue_cents"`
	}

	var rows []row
	err := q.db.WithContext(ctx).Raw(`
		SELECT
			oi.product_id,
			oi.product_name_snapshot AS product_name,
			SUM(oi.quantity)         AS quantity,
			SUM(oi.line_total)       AS revenue_cents
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE o.barbershop_id = ?
		  AND o.created_at >= ?
		  AND o.created_at < ?
		  AND (
		      o.status = 'paid'
		      OR EXISTS (
		          SELECT 1 FROM appointment_closures ac
		          WHERE ac.additional_order_id = o.id
		      )
		  )
		GROUP BY oi.product_id, oi.product_name_snapshot
		ORDER BY revenue_cents DESC
		LIMIT 5
	`, barbershopID, start, end).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]ProductRankItem, len(rows))
	for i, r := range rows {
		result[i] = ProductRankItem{
			ProductID:    r.ProductID,
			ProductName:  r.ProductName,
			Quantity:     r.Quantity,
			RevenueCents: r.RevenueCents,
		}
	}

	return result, nil
}

// ----------------------------------------------------------------
// Period helpers
// ----------------------------------------------------------------

func periodRange(period PeriodType, loc *time.Location) (startUTC, endUTC time.Time) {
	now := time.Now().In(loc)
	var localStart time.Time

	switch period {
	case PeriodDay:
		localStart = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	case PeriodWeek:
		// Week starts on Monday
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
	case PeriodDay:
		localEnd = localStart.AddDate(0, 0, 1)
	case PeriodWeek:
		localEnd = localStart.AddDate(0, 0, 7)
	case PeriodMonth:
		localEnd = localStart.AddDate(0, 1, 0)
	}

	return localStart.UTC(), localEnd.UTC()
}
