package impact

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
)

var ErrBarbershopNotFound = errors.New("barbershop not found")
var ErrInvalidPeriod = errors.New("invalid period, expected: week|month")

// Query is the read-only impact/ROI service.
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
	start, end := periodRange(period, loc)
	prevStart, prevEnd := prevPeriodRange(period, loc)

	dateFrom := start.In(loc).Format("2006-01-02")
	dateTo := end.Add(-time.Second).In(loc).Format("2006-01-02")

	revenue, err := q.loadRevenue(ctx, input.BarbershopID, start, end, prevStart, prevEnd)
	if err != nil {
		return nil, err
	}

	growth, err := q.loadGrowth(ctx, input.BarbershopID, start, end, loc, period)
	if err != nil {
		return nil, err
	}

	retention, err := q.loadRetention(ctx, input.BarbershopID, start, end, growth.ReturningClientsCount, growth.TotalActiveClients)
	if err != nil {
		return nil, err
	}

	losses, err := q.loadLosses(ctx, input.BarbershopID, start, end)
	if err != nil {
		return nil, err
	}

	usage, err := q.loadUsage(ctx, input.BarbershopID, start, end)
	if err != nil {
		return nil, err
	}

	indirect, err := q.loadIndirect(ctx, input.BarbershopID, start, end)
	if err != nil {
		return nil, err
	}

	roi, err := q.buildROI(ctx, input.BarbershopID, start, end, revenue, losses, indirect, usage, growth)
	if err != nil {
		return nil, err
	}

	return &ResponseDTO{
		Period:    string(period),
		DateFrom:  dateFrom,
		DateTo:    dateTo,
		Timezone:  shop.Timezone,
		Revenue:   revenue,
		Growth:    growth,
		Retention: retention,
		Losses:    losses,
		Usage:     usage,
		Indirect:  indirect,
		ROI:       roi,
	}, nil
}

// ----------------------------------------------------------------
// Revenue
// ----------------------------------------------------------------

func (q *Query) loadRevenue(ctx context.Context, barbershopID uint, start, end, prevStart, prevEnd time.Time) (RevenueDTO, error) {
	type revenueRow struct {
		ServicesCents int64 `gorm:"column:services_cents"`
		ClosuresCount int   `gorm:"column:closures_count"`
		OrdersCents   int64 `gorm:"column:orders_cents"`
	}

	loadPeriod := func(pStart, pEnd time.Time) (revenueRow, error) {
		var closureRes struct {
			ServicesCents int64 `gorm:"column:services_cents"`
			ClosuresCount int   `gorm:"column:closures_count"`
		}
		err := q.db.WithContext(ctx).Raw(`
			SELECT
				COALESCE(SUM(COALESCE(ac.final_amount_cents, ac.reference_amount_cents)), 0) AS services_cents,
				COUNT(*) AS closures_count
			FROM appointment_closures ac
			JOIN appointments a ON a.id = ac.appointment_id
			WHERE ac.barbershop_id = ?
			  AND a.start_time >= ?
			  AND a.start_time < ?
		`, barbershopID, pStart, pEnd).Scan(&closureRes).Error
		if err != nil {
			return revenueRow{}, err
		}

		var orderRes struct {
			OrdersCents int64 `gorm:"column:orders_cents"`
		}
		err = q.db.WithContext(ctx).Raw(`
			SELECT COALESCE(SUM(o.total_amount), 0) AS orders_cents
			FROM orders o
			WHERE o.barbershop_id = ?
			  AND o.status = 'paid'
			  AND o.created_at >= ?
			  AND o.created_at < ?
		`, barbershopID, pStart, pEnd).Scan(&orderRes).Error
		if err != nil {
			return revenueRow{}, err
		}

		return revenueRow{
			ServicesCents: closureRes.ServicesCents,
			ClosuresCount: closureRes.ClosuresCount,
			OrdersCents:   orderRes.OrdersCents,
		}, nil
	}

	current, err := loadPeriod(start, end)
	if err != nil {
		return RevenueDTO{}, err
	}

	previous, err := loadPeriod(prevStart, prevEnd)
	if err != nil {
		return RevenueDTO{}, err
	}

	currentTotal := current.ServicesCents + current.OrdersCents
	previousTotal := previous.ServicesCents + previous.OrdersCents

	var growthPercent float64
	if previousTotal > 0 {
		growthPercent = float64(currentTotal-previousTotal) / float64(previousTotal) * 100
	}

	var ticketAverage int64
	if current.ClosuresCount > 0 {
		ticketAverage = currentTotal / int64(current.ClosuresCount)
	}

	return RevenueDTO{
		CurrentCents:       currentTotal,
		PreviousCents:      previousTotal,
		GrowthPercent:      growthPercent,
		TicketAverageCents: ticketAverage,
	}, nil
}

// ----------------------------------------------------------------
// Growth
// ----------------------------------------------------------------

func (q *Query) loadGrowth(ctx context.Context, barbershopID uint, start, end time.Time, loc *time.Location, period PeriodType) (GrowthDTO, error) {
	// New clients: first ever appointment is within this period
	var newCount struct {
		Count int `gorm:"column:count"`
	}
	err := q.db.WithContext(ctx).Raw(`
		SELECT COUNT(DISTINCT a.client_id) AS count
		FROM appointments a
		WHERE a.barbershop_id = ?
		  AND a.client_id IS NOT NULL
		  AND a.start_time >= ?
		  AND a.start_time < ?
		  AND NOT EXISTS (
		    SELECT 1 FROM appointments a2
		    WHERE a2.client_id = a.client_id
		      AND a2.barbershop_id = a.barbershop_id
		      AND a2.start_time < ?
		  )
	`, barbershopID, start, end, start).Scan(&newCount).Error
	if err != nil {
		return GrowthDTO{}, err
	}

	// Returning clients: had appointment before and also in period
	var returningCount struct {
		Count int `gorm:"column:count"`
	}
	err = q.db.WithContext(ctx).Raw(`
		SELECT COUNT(DISTINCT a.client_id) AS count
		FROM appointments a
		WHERE a.barbershop_id = ?
		  AND a.client_id IS NOT NULL
		  AND a.start_time >= ?
		  AND a.start_time < ?
		  AND EXISTS (
		    SELECT 1 FROM appointments a2
		    WHERE a2.client_id = a.client_id
		      AND a2.barbershop_id = a.barbershop_id
		      AND a2.start_time < ?
		  )
	`, barbershopID, start, end, start).Scan(&returningCount).Error
	if err != nil {
		return GrowthDTO{}, err
	}

	// Total distinct clients active in the period
	var totalActive struct {
		Count int `gorm:"column:count"`
	}
	err = q.db.WithContext(ctx).Raw(`
		SELECT COUNT(DISTINCT a.client_id) AS count
		FROM appointments a
		WHERE a.barbershop_id = ?
		  AND a.client_id IS NOT NULL
		  AND a.start_time >= ?
		  AND a.start_time < ?
	`, barbershopID, start, end).Scan(&totalActive).Error
	if err != nil {
		return GrowthDTO{}, err
	}

	// Total appointments count
	var apptCount struct {
		Count int `gorm:"column:count"`
	}
	err = q.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) AS count
		FROM appointments
		WHERE barbershop_id = ?
		  AND start_time >= ?
		  AND start_time < ?
	`, barbershopID, start, end).Scan(&apptCount).Error
	if err != nil {
		return GrowthDTO{}, err
	}

	// Trend
	trend, err := q.loadTrend(ctx, barbershopID, start, end, loc, period)
	if err != nil {
		return GrowthDTO{}, err
	}

	return GrowthDTO{
		NewClientsCount:       newCount.Count,
		ReturningClientsCount: returningCount.Count,
		TotalActiveClients:    totalActive.Count,
		AppointmentsCount:     apptCount.Count,
		Trend:                 trend,
	}, nil
}

func (q *Query) loadTrend(ctx context.Context, barbershopID uint, start, end time.Time, loc *time.Location, period PeriodType) ([]TrendPointDTO, error) {
	tzName := loc.String()

	if period == PeriodWeek {
		type trendRow struct {
			DOW          int   `gorm:"column:dow"`
			Count        int   `gorm:"column:count"`
			RevenueCents int64 `gorm:"column:revenue_cents"`
		}

		var rows []trendRow
		err := q.db.WithContext(ctx).Raw(`
			SELECT
				EXTRACT(DOW FROM a.start_time AT TIME ZONE ?) AS dow,
				COUNT(a.id) AS count,
				COALESCE(SUM(COALESCE(ac.final_amount_cents, ac.reference_amount_cents)), 0) AS revenue_cents
			FROM appointments a
			LEFT JOIN appointment_closures ac ON ac.appointment_id = a.id
			WHERE a.barbershop_id = ?
			  AND a.start_time >= ?
			  AND a.start_time < ?
			GROUP BY dow
			ORDER BY dow
		`, tzName, barbershopID, start, end).Scan(&rows).Error
		if err != nil {
			return nil, err
		}

		// DOW: 0=Sun, 1=Mon, 2=Tue, 3=Wed, 4=Thu, 5=Fri, 6=Sat
		// Week order Mon-Sun: 1,2,3,4,5,6,0
		dowLabels := map[int]string{
			1: "Seg",
			2: "Ter",
			3: "Qua",
			4: "Qui",
			5: "Sex",
			6: "Sáb",
			0: "Dom",
		}
		weekOrder := []int{1, 2, 3, 4, 5, 6, 0}

		// Build lookup map
		byDOW := make(map[int]trendRow)
		for _, r := range rows {
			byDOW[r.DOW] = r
		}

		trend := make([]TrendPointDTO, 0, 7)
		for _, dow := range weekOrder {
			r := byDOW[dow]
			trend = append(trend, TrendPointDTO{
				Label:        dowLabels[dow],
				Count:        r.Count,
				RevenueCents: r.RevenueCents,
			})
		}
		return trend, nil
	}

	// Month period: group by week number
	type weekRow struct {
		WeekNum      float64 `gorm:"column:week_num"`
		Count        int     `gorm:"column:count"`
		RevenueCents int64   `gorm:"column:revenue_cents"`
	}

	var rows []weekRow
	err := q.db.WithContext(ctx).Raw(`
		SELECT
			EXTRACT(WEEK FROM a.start_time AT TIME ZONE ?) AS week_num,
			COUNT(a.id) AS count,
			COALESCE(SUM(COALESCE(ac.final_amount_cents, ac.reference_amount_cents)), 0) AS revenue_cents
		FROM appointments a
		LEFT JOIN appointment_closures ac ON ac.appointment_id = a.id
		WHERE a.barbershop_id = ?
		  AND a.start_time >= ?
		  AND a.start_time < ?
		GROUP BY week_num
		ORDER BY week_num
	`, tzName, barbershopID, start, end).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	trend := make([]TrendPointDTO, 0, len(rows))
	for i, r := range rows {
		trend = append(trend, TrendPointDTO{
			Label:        fmt.Sprintf("Sem %d", i+1),
			Count:        r.Count,
			RevenueCents: r.RevenueCents,
		})
	}
	return trend, nil
}

// ----------------------------------------------------------------
// Retention
// ----------------------------------------------------------------

func (q *Query) loadRetention(ctx context.Context, barbershopID uint, start, end time.Time, returningCount, totalActive int) (RetentionDTO, error) {
	var returnRate float64
	if totalActive > 0 {
		returnRate = float64(returningCount) / float64(totalActive) * 100
	}

	var atRisk struct {
		Count int `gorm:"column:count"`
	}
	err := q.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) AS count
		FROM client_metrics
		WHERE barbershop_id = ? AND category = 'at_risk'
	`, barbershopID).Scan(&atRisk).Error
	if err != nil {
		return RetentionDTO{}, err
	}

	var trusted struct {
		Count int `gorm:"column:count"`
	}
	err = q.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) AS count
		FROM client_metrics
		WHERE barbershop_id = ? AND category = 'trusted'
	`, barbershopID).Scan(&trusted).Error
	if err != nil {
		return RetentionDTO{}, err
	}

	var inactive struct {
		Count int `gorm:"column:count"`
	}
	err = q.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) AS count
		FROM (
			SELECT client_id
			FROM appointments
			WHERE barbershop_id = ?
			  AND client_id IS NOT NULL
			  AND status NOT IN ('cancelled')
			GROUP BY client_id
			HAVING MAX(start_time) < NOW() - INTERVAL '60 days'
		) sub
	`, barbershopID).Scan(&inactive).Error
	if err != nil {
		return RetentionDTO{}, err
	}

	return RetentionDTO{
		ReturnRatePercent: returnRate,
		AtRiskCount:       atRisk.Count,
		TrustedCount:      trusted.Count,
		InactiveCount:     inactive.Count,
	}, nil
}

// ----------------------------------------------------------------
// Losses
// ----------------------------------------------------------------

func (q *Query) loadLosses(ctx context.Context, barbershopID uint, start, end time.Time) (LossesDTO, error) {
	var noShow struct {
		AmountCents int64 `gorm:"column:amount_cents"`
		Count       int   `gorm:"column:count"`
	}
	err := q.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(SUM(bs.price), 0) AS amount_cents,
			COUNT(a.id) AS count
		FROM appointments a
		JOIN barbershop_services bs ON bs.id = a.barber_product_id
		WHERE a.barbershop_id = ?
		  AND a.status = 'no_show'
		  AND a.start_time >= ?
		  AND a.start_time < ?
	`, barbershopID, start, end).Scan(&noShow).Error
	if err != nil {
		return LossesDTO{}, err
	}

	var cancel struct {
		AmountCents int64 `gorm:"column:amount_cents"`
		Count       int   `gorm:"column:count"`
	}
	err = q.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(SUM(bs.price), 0) AS amount_cents,
			COUNT(a.id) AS count
		FROM appointments a
		JOIN barbershop_services bs ON bs.id = a.barber_product_id
		WHERE a.barbershop_id = ?
		  AND a.status = 'cancelled'
		  AND a.start_time >= ?
		  AND a.start_time < ?
	`, barbershopID, start, end).Scan(&cancel).Error
	if err != nil {
		return LossesDTO{}, err
	}

	return LossesDTO{
		TotalCents:        noShow.AmountCents + cancel.AmountCents,
		NoShowCents:       noShow.AmountCents,
		NoShowCount:       noShow.Count,
		CancellationCents: cancel.AmountCents,
		CancellationCount: cancel.Count,
	}, nil
}

// ----------------------------------------------------------------
// Usage
// ----------------------------------------------------------------

func (q *Query) loadUsage(ctx context.Context, barbershopID uint, start, end time.Time) (UsageDTO, error) {
	type statusRow struct {
		Status string `gorm:"column:status"`
		Count  int    `gorm:"column:count"`
	}

	var rows []statusRow
	err := q.db.WithContext(ctx).Raw(`
		SELECT status, COUNT(*) AS count
		FROM appointments
		WHERE barbershop_id = ?
		  AND start_time >= ?
		  AND start_time < ?
		GROUP BY status
	`, barbershopID, start, end).Scan(&rows).Error
	if err != nil {
		return UsageDTO{}, err
	}

	var total, completed, noShow, cancelled int
	for _, r := range rows {
		total += r.Count
		switch r.Status {
		case "completed":
			completed = r.Count
		case "no_show":
			noShow = r.Count
		case "cancelled":
			cancelled = r.Count
		}
	}

	var closuresCount struct {
		Count int `gorm:"column:count"`
	}
	err = q.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) AS count
		FROM appointment_closures ac
		JOIN appointments a ON a.id = ac.appointment_id
		WHERE ac.barbershop_id = ?
		  AND a.start_time >= ?
		  AND a.start_time < ?
	`, barbershopID, start, end).Scan(&closuresCount).Error
	if err != nil {
		return UsageDTO{}, err
	}

	var adjustmentsCount struct {
		Count int `gorm:"column:count"`
	}
	err = q.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) AS count
		FROM closure_adjustments ca
		JOIN appointment_closures ac ON ac.id = ca.closure_id
		JOIN appointments a ON a.id = ac.appointment_id
		WHERE ac.barbershop_id = ?
		  AND a.start_time >= ?
		  AND a.start_time < ?
	`, barbershopID, start, end).Scan(&adjustmentsCount).Error
	if err != nil {
		return UsageDTO{}, err
	}

	denominator := completed + noShow + cancelled
	var attendanceRate float64
	if denominator > 0 {
		attendanceRate = float64(completed) / float64(denominator) * 100
	}

	var closureRate float64
	if completed > 0 {
		closureRate = float64(closuresCount.Count) / float64(completed) * 100
	}

	return UsageDTO{
		TotalAppointments:     total,
		CompletedCount:        completed,
		AttendanceRatePercent: attendanceRate,
		ClosuresCount:         closuresCount.Count,
		ClosureRatePercent:    closureRate,
		AdjustmentsCount:      adjustmentsCount.Count,
	}, nil
}

// ----------------------------------------------------------------
// Indirect
// ----------------------------------------------------------------

func (q *Query) loadIndirect(ctx context.Context, barbershopID uint, start, end time.Time) (IndirectDTO, error) {
	// Additional sales from closures with additional_order_id
	var additionalSales struct {
		TotalCents int64 `gorm:"column:total_cents"`
		Count      int   `gorm:"column:count"`
	}
	err := q.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(SUM(o.total_amount), 0) AS total_cents,
			COUNT(o.id) AS count
		FROM orders o
		JOIN appointment_closures ac ON ac.additional_order_id = o.id
		JOIN appointments a ON a.id = ac.appointment_id
		WHERE a.barbershop_id = ?
		  AND a.start_time >= ?
		  AND a.start_time < ?
	`, barbershopID, start, end).Scan(&additionalSales).Error
	if err != nil {
		return IndirectDTO{}, err
	}

	// Active subscriptions
	var activeSubs struct {
		Count int `gorm:"column:count"`
	}
	err = q.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) AS count
		FROM subscriptions
		WHERE barbershop_id = ? AND status = 'active'
	`, barbershopID).Scan(&activeSubs).Error
	if err != nil {
		return IndirectDTO{}, err
	}

	// Suggestion conversion rate
	var suggRow struct {
		Converted int `gorm:"column:converted"`
		Removed   int `gorm:"column:removed"`
	}
	err = q.db.WithContext(ctx).Raw(`
		SELECT
			COUNT(*) FILTER (WHERE ac.suggestion_removed = false) AS converted,
			COUNT(*) FILTER (WHERE ac.suggestion_removed = true) AS removed
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
		return IndirectDTO{}, err
	}

	var suggConversionRate float64
	suggTotal := suggRow.Converted + suggRow.Removed
	if suggTotal > 0 {
		suggConversionRate = float64(suggRow.Converted) / float64(suggTotal) * 100
	}

	return IndirectDTO{
		AdditionalSalesCents:     additionalSales.TotalCents,
		AdditionalSalesCount:     additionalSales.Count,
		ActiveSubscriptionsCount: activeSubs.Count,
		SuggestionConversionRate: suggConversionRate,
		UpsellCapturedCents:      additionalSales.TotalCents,
	}, nil
}

// ----------------------------------------------------------------
// ROI
// ----------------------------------------------------------------

func (q *Query) buildROI(ctx context.Context, barbershopID uint, start, end time.Time, revenue RevenueDTO, losses LossesDTO, indirect IndirectDTO, usage UsageDTO, growth GrowthDTO) (ROIDTO, error) {
	// Subscription value: sum of closure amounts covered by subscription
	var subValue struct {
		TotalCents int64 `gorm:"column:total_cents"`
	}
	err := q.db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(COALESCE(ac.final_amount_cents, ac.reference_amount_cents)), 0) AS total_cents
		FROM appointment_closures ac
		JOIN appointments a ON a.id = ac.appointment_id
		WHERE ac.barbershop_id = ?
		  AND ac.subscription_covered = true
		  AND a.start_time >= ?
		  AND a.start_time < ?
	`, barbershopID, start, end).Scan(&subValue).Error
	if err != nil {
		return ROIDTO{}, err
	}

	valueGenerated := revenue.CurrentCents + indirect.AdditionalSalesCents
	lossesMitigatedNote := fmt.Sprintf("R$ %.2f em perdas identificadas no período", float64(losses.TotalCents)/100)

	justificationNote := fmt.Sprintf(
		"No período, o sistema registrou R$ %.2f em receita realizada, %d atendimentos com taxa de presença de %.1f%% e %d clientes ativos. "+
			"Foram identificadas perdas de R$ %.2f (no-show e cancelamentos). "+
			"Ganhos indiretos de R$ %.2f em vendas adicionais e R$ %.2f cobertos por assinaturas.",
		float64(revenue.CurrentCents)/100,
		usage.TotalAppointments,
		usage.AttendanceRatePercent,
		growth.TotalActiveClients,
		float64(losses.TotalCents)/100,
		float64(indirect.AdditionalSalesCents)/100,
		float64(subValue.TotalCents)/100,
	)

	return ROIDTO{
		ValueGeneratedCents:    valueGenerated,
		LossesMitigatedNote:    lossesMitigatedNote,
		SubscriptionValueCents: subValue.TotalCents,
		JustificationNote:      justificationNote,
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

func prevPeriodRange(period PeriodType, loc *time.Location) (startUTC, endUTC time.Time) {
	start, end := periodRange(period, loc)
	dur := end.Sub(start)
	return start.Add(-dur), start
}
