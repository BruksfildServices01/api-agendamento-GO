package daypanel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/query/shared"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
)

var ErrInvalidDate = errors.New("invalid date format, expected YYYY-MM-DD")
var ErrBarbershopNotFound = errors.New("barbershop not found")

// Input parameters for the day panel query.
type Input struct {
	BarbershopID uint
	BarberID     uint   // 0 = all barbers of the shop
	Date         string // YYYY-MM-DD in the barbershop's local timezone; empty = today
}

// Query is the read-only service for the day panel.
type Query struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Query {
	return &Query{db: db}
}

// ----------------------------------------------------------------
// Execute — main entry point
// ----------------------------------------------------------------

func (q *Query) Execute(ctx context.Context, input Input) (*ResponseDTO, error) {
	// 1. Load barbershop (timezone is source of truth)
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

	// 2. Parse date in shop timezone → UTC range
	loc := timezone.Location(shop.Timezone)
	localDate, err := parseLocalDate(input.Date, loc)
	if err != nil {
		return nil, ErrInvalidDate
	}

	startUTC := localDate.UTC()
	endUTC := localDate.Add(24 * time.Hour).UTC()
	dateStr := localDate.Format("2006-01-02")

	// 3. Load appointments with client, service and payment in one query
	rows, err := q.loadAppointmentRows(ctx, input.BarbershopID, input.BarberID, startUTC, endUTC)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return &ResponseDTO{
			Date:     dateStr,
			Timezone: shop.Timezone,
			Cards:    []CardDTO{},
			Summary:  SummaryDTO{},
		}, nil
	}

	// 4. Collect IDs for batch queries
	clientIDs := collectClientIDs(rows)
	serviceIDs := collectServiceIDs(rows)

	// 5. Batch load related data (no N+1)
	subscriptionsByClient, err := q.loadSubscriptions(ctx, input.BarbershopID, clientIDs)
	if err != nil {
		return nil, err
	}

	coveredServicesByPlan, err := q.loadPlanServiceCoverage(ctx, subscriptionsByClient, serviceIDs)
	if err != nil {
		return nil, err
	}

	suggestionsByService, err := q.loadSuggestions(ctx, input.BarbershopID, serviceIDs)
	if err != nil {
		return nil, err
	}

	prePaidOrdersByClient, err := q.loadPrePaidOrders(ctx, input.BarbershopID, clientIDs, startUTC, endUTC)
	if err != nil {
		return nil, err
	}

	// 6. Assemble cards
	cards := make([]CardDTO, 0, len(rows))
	for _, row := range rows {
		card := assembleCard(row, subscriptionsByClient, coveredServicesByPlan, suggestionsByService, prePaidOrdersByClient)
		cards = append(cards, card)
	}

	return &ResponseDTO{
		Date:     dateStr,
		Timezone: shop.Timezone,
		Cards:    cards,
		Summary:  buildSummary(cards),
	}, nil
}

// ----------------------------------------------------------------
// appointmentRow — raw result from the main query
// ----------------------------------------------------------------

type appointmentRow struct {
	AppointmentID uint      `gorm:"column:appointment_id"`
	StartTime     time.Time `gorm:"column:start_time"`
	EndTime       time.Time `gorm:"column:end_time"`
	Status        string    `gorm:"column:status"`
	CreatedBy     string    `gorm:"column:created_by"`
	Notes         string    `gorm:"column:notes"`

	ClientID       *uint  `gorm:"column:client_id"`
	ClientName     string `gorm:"column:client_name"`
	ClientPhone    string `gorm:"column:client_phone"`
	ClientEmail    string `gorm:"column:client_email"`
	ClientCategory string `gorm:"column:client_category"`

	ServiceID       *uint  `gorm:"column:service_id"`
	ServiceName     string `gorm:"column:service_name"`
	ServiceDuration int    `gorm:"column:service_duration_min"`
	ServicePrice    int64  `gorm:"column:service_price_cents"`

	PaymentID            *uint      `gorm:"column:payment_id"`
	PaymentStatus        string     `gorm:"column:payment_status"`
	PaymentAmount        int64      `gorm:"column:payment_amount_cents"`
	PaymentPaidAt        *time.Time `gorm:"column:payment_paid_at"`
	ClosurePaymentMethod string     `gorm:"column:closure_payment_method"`
}

func (q *Query) loadAppointmentRows(ctx context.Context, barbershopID, barberID uint, startUTC, endUTC time.Time) ([]appointmentRow, error) {
	sql := `
		SELECT
			a.id            AS appointment_id,
			a.start_time,
			a.end_time,
			a.status,
			a.created_by,
			COALESCE(a.notes, '') AS notes,

			c.id            AS client_id,
			COALESCE(c.name, '')  AS client_name,
			COALESCE(c.phone, '') AS client_phone,
			COALESCE(c.email, '') AS client_email,
			COALESCE(cm.category::text, 'new') AS client_category,

			bs.id           AS service_id,
			COALESCE(bs.name, '')  AS service_name,
			COALESCE(bs.duration_min, 0) AS service_duration_min,
			COALESCE(bs.price, 0) AS service_price_cents,

			p.id            AS payment_id,
			COALESCE(p.status::text, 'none') AS payment_status,
			COALESCE(p.amount, 0)  AS payment_amount_cents,
			p.paid_at       AS payment_paid_at,

			COALESCE(ac.payment_method, '') AS closure_payment_method

		FROM appointments a
		LEFT JOIN clients c
			ON c.id = a.client_id
		LEFT JOIN client_metrics cm
			ON cm.client_id = a.client_id
			AND cm.barbershop_id = a.barbershop_id
		LEFT JOIN barbershop_services bs
			ON bs.id = a.barber_product_id
		LEFT JOIN payments p
			ON p.appointment_id = a.id
			AND p.barbershop_id = a.barbershop_id
		LEFT JOIN appointment_closures ac
			ON ac.appointment_id = a.id

		WHERE a.barbershop_id = ?
		  AND a.start_time >= ?
		  AND a.start_time < ?
	`

	args := []any{barbershopID, startUTC, endUTC}

	if barberID > 0 {
		sql += " AND a.barber_id = ?"
		args = append(args, barberID)
	}

	sql += " ORDER BY a.start_time ASC"

	var rows []appointmentRow
	if err := q.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}

	return rows, nil
}

// ----------------------------------------------------------------
// Subscriptions batch
// ----------------------------------------------------------------

type subscriptionRow struct {
	ClientID     uint      `gorm:"column:client_id"`
	PlanID       uint      `gorm:"column:plan_id"`
	PlanName     string    `gorm:"column:plan_name"`
	CutsUsed     int       `gorm:"column:cuts_used"`
	CutsIncluded int       `gorm:"column:cuts_included"`
	ValidUntil   time.Time `gorm:"column:valid_until"`
}

func (q *Query) loadSubscriptions(ctx context.Context, barbershopID uint, clientIDs []int64) (map[uint]subscriptionRow, error) {
	result := make(map[uint]subscriptionRow)
	if len(clientIDs) == 0 {
		return result, nil
	}

	var rows []subscriptionRow
	err := q.db.WithContext(ctx).Raw(`
		SELECT
			s.client_id,
			p.id   AS plan_id,
			p.name AS plan_name,
			s.cuts_used_in_period AS cuts_used,
			p.cuts_included,
			s.current_period_end  AS valid_until
		FROM subscriptions s
		JOIN plans p ON p.id = s.plan_id
		WHERE s.barbershop_id = ?
		  AND s.client_id = ANY(`+pgIntArray(clientIDs)+`)
		  AND `+shared.ActiveSubscriptionSQL,
		barbershopID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		result[r.ClientID] = r
	}

	return result, nil
}

// ----------------------------------------------------------------
// Plan service coverage batch
// ----------------------------------------------------------------

// loadPlanServiceCoverage returns a map of planID → set of covered serviceIDs.
func (q *Query) loadPlanServiceCoverage(ctx context.Context, subscriptions map[uint]subscriptionRow, serviceIDs []int64) (map[uint]map[uint]bool, error) {
	result := make(map[uint]map[uint]bool)
	if len(subscriptions) == 0 || len(serviceIDs) == 0 {
		return result, nil
	}

	planIDs := make([]int64, 0, len(subscriptions))
	seen := make(map[uint]bool)
	for _, sub := range subscriptions {
		if !seen[sub.PlanID] {
			planIDs = append(planIDs, int64(sub.PlanID))
			seen[sub.PlanID] = true
		}
	}

	type coverageRow struct {
		PlanID    uint `gorm:"column:plan_id"`
		ServiceID uint `gorm:"column:service_id"`
	}

	var rows []coverageRow
	err := q.db.WithContext(ctx).Raw(`
		SELECT plan_id, service_id
		FROM plan_services
		WHERE plan_id = ANY(`+pgIntArray(planIDs)+`)
		  AND service_id = ANY(`+pgIntArray(serviceIDs)+`)
	`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		if result[r.PlanID] == nil {
			result[r.PlanID] = make(map[uint]bool)
		}
		result[r.PlanID][r.ServiceID] = true
	}

	return result, nil
}

// ----------------------------------------------------------------
// Suggestions batch
// ----------------------------------------------------------------

type suggestionRow struct {
	ServiceID   uint   `gorm:"column:service_id"`
	ProductID   uint   `gorm:"column:product_id"`
	ProductName string `gorm:"column:product_name"`
	PriceCents  int64  `gorm:"column:price_cents"`
}

func (q *Query) loadSuggestions(ctx context.Context, barbershopID uint, serviceIDs []int64) (map[uint]suggestionRow, error) {
	result := make(map[uint]suggestionRow)
	if len(serviceIDs) == 0 {
		return result, nil
	}

	var rows []suggestionRow
	err := q.db.WithContext(ctx).Raw(`
		SELECT
			ssp.service_id,
			prod.id    AS product_id,
			prod.name  AS product_name,
			prod.price AS price_cents
		FROM service_suggested_products ssp
		JOIN products prod ON prod.id = ssp.product_id
		WHERE ssp.barbershop_id = ?
		  AND ssp.service_id = ANY(`+pgIntArray(serviceIDs)+`)
		  AND ssp.active = true
	`, barbershopID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		result[r.ServiceID] = r
	}

	return result, nil
}

// ----------------------------------------------------------------
// Pre-paid orders batch
// ----------------------------------------------------------------

type prePaidOrderRow struct {
	ClientID   uint       `gorm:"column:client_id"`
	OrderID    uint       `gorm:"column:order_id"`
	TotalCents int64      `gorm:"column:total_cents"`
	ItemsCount int        `gorm:"column:items_count"`
	PaidAt     *time.Time `gorm:"column:paid_at"`
}

func (q *Query) loadPrePaidOrders(ctx context.Context, barbershopID uint, clientIDs []int64, startUTC, endUTC time.Time) (map[uint]prePaidOrderRow, error) {
	result := make(map[uint]prePaidOrderRow)
	if len(clientIDs) == 0 {
		return result, nil
	}

	var rows []prePaidOrderRow
	err := q.db.WithContext(ctx).Raw(`
		SELECT
			o.client_id,
			o.id           AS order_id,
			o.total_amount AS total_cents,
			COUNT(oi.id)   AS items_count,
			p.paid_at
		FROM orders o
		JOIN payments p
			ON p.order_id = o.id
			AND p.barbershop_id = o.barbershop_id
		LEFT JOIN order_items oi ON oi.order_id = o.id
		WHERE o.barbershop_id = ?
		  AND o.client_id = ANY(`+pgIntArray(clientIDs)+`)
		  AND o.status = 'paid'
		  AND o.created_at >= ?
		  AND o.created_at < ?
		GROUP BY o.client_id, o.id, o.total_amount, p.paid_at
		ORDER BY o.created_at DESC
	`, barbershopID, startUTC, endUTC).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	// keep only the most recent paid order per client
	for _, r := range rows {
		if _, exists := result[r.ClientID]; !exists {
			result[r.ClientID] = r
		}
	}

	return result, nil
}

// ----------------------------------------------------------------
// Card assembly
// ----------------------------------------------------------------

func assembleCard(
	row appointmentRow,
	subscriptions map[uint]subscriptionRow,
	coveredServices map[uint]map[uint]bool,
	suggestions map[uint]suggestionRow,
	prePaidOrders map[uint]prePaidOrderRow,
) CardDTO {
	card := CardDTO{
		AppointmentID: row.AppointmentID,
		StartTime:     row.StartTime,
		EndTime:       row.EndTime,
		Status:        row.Status,
		CreatedBy:     row.CreatedBy,
		Notes:         row.Notes,
		Payment: PaymentDTO{
			Status:      row.PaymentStatus,
			AmountCents: row.PaymentAmount,
			PaidAt:      row.PaymentPaidAt,
			Method:      row.ClosurePaymentMethod,
		},
	}

	// Client
	if row.ClientID != nil {
		card.Client = ClientDTO{
			ID:       *row.ClientID,
			Name:     row.ClientName,
			Phone:    row.ClientPhone,
			Email:    row.ClientEmail,
			Category: row.ClientCategory,
		}
	}

	// Service
	if row.ServiceID != nil {
		card.Service = ServiceDTO{
			ID:          *row.ServiceID,
			Name:        row.ServiceName,
			DurationMin: row.ServiceDuration,
			PriceCents:  row.ServicePrice,
		}
	}

	// Suggestion (item previsto)
	if row.ServiceID != nil {
		if sug, ok := suggestions[*row.ServiceID]; ok {
			card.Suggestion = &SuggestionDTO{
				ProductID:   sug.ProductID,
				ProductName: sug.ProductName,
				PriceCents:  sug.PriceCents,
			}
		}
	}

	// Pre-paid order (item pago antecipadamente)
	if row.ClientID != nil {
		if order, ok := prePaidOrders[*row.ClientID]; ok {
			card.PrePaidOrder = &PrePaidOrderDTO{
				OrderID:    order.OrderID,
				TotalCents: order.TotalCents,
				ItemsCount: order.ItemsCount,
				PaidAt:     order.PaidAt,
			}
		}
	}

	// Subscription
	if row.ClientID != nil {
		if sub, ok := subscriptions[*row.ClientID]; ok {
			covered := false
			if row.ServiceID != nil {
				if planServices, ok := coveredServices[sub.PlanID]; ok {
					covered = planServices[*row.ServiceID]
				}
			}
			card.Subscription = &SubscriptionDTO{
				PlanID:         sub.PlanID,
				PlanName:       sub.PlanName,
				CutsUsed:       sub.CutsUsed,
				CutsIncluded:   sub.CutsIncluded,
				ValidUntil:     sub.ValidUntil,
				ServiceCovered: covered,
			}
		}
	}

	// Flags (pre-computed operational alerts)
	card.Flags = FlagsDTO{
		AwaitingPayment: row.Status == "awaiting_payment",
		HasPrePaidItems: card.PrePaidOrder != nil,
		HasSuggestion:   card.Suggestion != nil,
		IsAtRisk:        row.ClientCategory == "at_risk",
		HasSubscription: card.Subscription != nil,
	}

	return card
}

// ----------------------------------------------------------------
// Summary
// ----------------------------------------------------------------

func buildSummary(cards []CardDTO) SummaryDTO {
	s := SummaryDTO{Total: len(cards)}
	for _, c := range cards {
		switch c.Status {
		case "scheduled":
			s.Scheduled++
		case "awaiting_payment":
			s.AwaitingPayment++
		case "completed":
			s.Completed++
		case "cancelled":
			s.Cancelled++
		case "no_show":
			s.NoShow++
		}
	}
	return s
}

// ----------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------

func parseLocalDate(dateStr string, loc *time.Location) (time.Time, error) {
	if dateStr == "" {
		now := time.Now().In(loc)
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc), nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

// pgIntArray builds a safe PostgreSQL ARRAY literal from a []int64.
// Avoids driver serialization issues with pgx/v5 + GORM.
// Safe because values are guaranteed integers — no SQL injection possible.
func pgIntArray(ids []int64) string {
	if len(ids) == 0 {
		return "ARRAY[]::bigint[]"
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	return "ARRAY[" + strings.Join(parts, ",") + "]::bigint[]"
}

func collectClientIDs(rows []appointmentRow) []int64 {
	seen := make(map[uint]bool)
	ids := make([]int64, 0)
	for _, r := range rows {
		if r.ClientID != nil && !seen[*r.ClientID] {
			ids = append(ids, int64(*r.ClientID))
			seen[*r.ClientID] = true
		}
	}
	return ids
}

func collectServiceIDs(rows []appointmentRow) []int64 {
	seen := make(map[uint]bool)
	ids := make([]int64, 0)
	for _, r := range rows {
		if r.ServiceID != nil && !seen[*r.ServiceID] {
			ids = append(ids, int64(*r.ServiceID))
			seen[*r.ServiceID] = true
		}
	}
	return ids
}
