package crm

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	domainMetrics "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
	"github.com/BruksfildServices01/barber-scheduler/internal/query/shared"
)

var ErrClientNotFound = errors.New("client not found")

// Query is the read-only CRM service for a single client.
type Query struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Query {
	return &Query{db: db}
}

// ----------------------------------------------------------------
// Execute
// ----------------------------------------------------------------

func (q *Query) Execute(ctx context.Context, barbershopID, clientID uint) (*ResponseDTO, error) {
	// 1–3. Run all three independent queries in parallel.
	// None depends on the output of another during the fetch phase; post-processing
	// (category resolution, flags, policy) happens after all results are collected.
	var client struct {
		ID    uint   `gorm:"column:id"`
		Name  string `gorm:"column:name"`
		Phone string `gorm:"column:phone"`
		Email string `gorm:"column:email"`
	}
	var m struct {
		TotalAppointments           int        `gorm:"column:total_appointments"`
		CompletedAppointments       int        `gorm:"column:completed_appointments"`
		CancelledAppointments       int        `gorm:"column:cancelled_appointments"`
		LateCancelledAppointments   int        `gorm:"column:late_cancelled_appointments"`
		NoShowAppointments          int        `gorm:"column:no_show_appointments"`
		RescheduledAppointments     int        `gorm:"column:rescheduled_appointments"`
		LateRescheduledAppointments int        `gorm:"column:late_rescheduled_appointments"`
		TotalSpent                  int64      `gorm:"column:total_spent"`
		FirstAppointmentAt          *time.Time `gorm:"column:first_appointment_at"`
		LastAppointmentAt           *time.Time `gorm:"column:last_appointment_at"`
		LastCompletedAt             *time.Time `gorm:"column:last_completed_at"`
		Category                    string     `gorm:"column:category"`
		CategorySource              string     `gorm:"column:category_source"`
		ManualCategoryExpiresAt     *time.Time `gorm:"column:manual_category_expires_at"`
	}
	var subRow struct {
		PlanID       uint      `gorm:"column:plan_id"`
		PlanName     string    `gorm:"column:plan_name"`
		CutsUsed     int       `gorm:"column:cuts_used"`
		CutsIncluded int       `gorm:"column:cuts_included"`
		ValidUntil   time.Time `gorm:"column:valid_until"`
	}
	metricsFound := true

	clientCh  := make(chan error, 1)
	metricsCh := make(chan error, 1)
	subCh     := make(chan error, 1)

	go func() {
		clientCh <- q.db.WithContext(ctx).
			Table("clients").
			Select("id, name, phone, email").
			Where("id = ? AND barbershop_id = ?", clientID, barbershopID).
			First(&client).Error
	}()

	go func() {
		err := q.db.WithContext(ctx).
			Table("client_metrics").
			Where("client_id = ? AND barbershop_id = ?", clientID, barbershopID).
			First(&m).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			metricsFound = false
			err = nil
		}
		metricsCh <- err
	}()

	go func() {
		subCh <- q.db.WithContext(ctx).Raw(`
			SELECT s.plan_id, p.name AS plan_name,
			       s.cuts_used_in_period AS cuts_used,
			       p.cuts_included,
			       s.current_period_end AS valid_until
			FROM subscriptions s
			JOIN plans p ON p.id = s.plan_id
			WHERE s.barbershop_id = ?
			  AND s.client_id = ?
			  AND `+shared.ActiveSubscriptionSQL+`
			LIMIT 1
		`, barbershopID, clientID).Scan(&subRow).Error
	}()

	// Always drain all channels before returning any error.
	// Channel receives happen-after the goroutine sends, guaranteeing memory
	// visibility of client, m, subRow, and metricsFound without additional sync.
	clientErr  := <-clientCh
	metricsErr := <-metricsCh
	subErr     := <-subCh

	if clientErr != nil {
		if errors.Is(clientErr, gorm.ErrRecordNotFound) {
			return nil, ErrClientNotFound
		}
		return nil, clientErr
	}
	if metricsErr != nil { return nil, metricsErr }
	// subErr is intentionally not checked: the original implementation silently
	// discarded subscription query errors (treating them as "no active subscription").
	// A transient failure here must not abort the CRM request.
	_ = subErr

	// 4. Resolve category (apply classifier for auto, respect manual if not expired)
	category := domainMetrics.CategoryNew
	categorySource := "auto"

	if metricsFound {
		dm := &domainMetrics.ClientMetrics{
			TotalAppointments:           m.TotalAppointments,
			CompletedAppointments:       m.CompletedAppointments,
			CancelledAppointments:       m.CancelledAppointments,
			LateCancelledAppointments:   m.LateCancelledAppointments,
			NoShowAppointments:          m.NoShowAppointments,
			RescheduledAppointments:     m.RescheduledAppointments,
			LateRescheduledAppointments: m.LateRescheduledAppointments,
			LastCompletedAt:             m.LastCompletedAt,
			ManualCategoryExpiresAt:     m.ManualCategoryExpiresAt,
			CategorySource:              domainMetrics.CategorySource(m.CategorySource),
			Category:                    domainMetrics.ClientCategory(m.Category),
		}

		if dm.CategorySource == domainMetrics.CategorySourceManual {
			if dm.ManualCategoryExpiresAt == nil || time.Now().UTC().Before(*dm.ManualCategoryExpiresAt) {
				category = dm.Category
				categorySource = "manual"
			} else {
				category = domainMetrics.Classify(dm)
			}
		} else {
			category = domainMetrics.Classify(dm)
		}
	}

	// 5. Build subscription DTO (subRow.PlanID == 0 means no active subscription found)
	var sub *SubscriptionDTO
	if subRow.PlanID != 0 {
		sub = &SubscriptionDTO{
			PlanID:       subRow.PlanID,
			PlanName:     subRow.PlanName,
			CutsUsed:     subRow.CutsUsed,
			CutsIncluded: subRow.CutsIncluded,
			ValidUntil:   subRow.ValidUntil,
		}
	}

	// 6. Compute metrics DTO
	var attendanceRate float64
	if metricsFound && m.TotalAppointments > 0 {
		attendanceRate = float64(m.CompletedAppointments) / float64(m.TotalAppointments)
	}

	metricsDTO := MetricsDTO{
		AttendanceRate: attendanceRate,
	}
	if metricsFound {
		metricsDTO.TotalAppointments = m.TotalAppointments
		metricsDTO.Completed = m.CompletedAppointments
		metricsDTO.Cancelled = m.CancelledAppointments
		metricsDTO.LateCancelled = m.LateCancelledAppointments
		metricsDTO.NoShow = m.NoShowAppointments
		metricsDTO.Rescheduled = m.RescheduledAppointments
		metricsDTO.LateRescheduled = m.LateRescheduledAppointments
		metricsDTO.TotalSpentCents = m.TotalSpent
		metricsDTO.FirstAppointmentAt = m.FirstAppointmentAt
		metricsDTO.LastAppointmentAt = m.LastAppointmentAt
		metricsDTO.LastCompletedAt = m.LastCompletedAt
	}

	// 7. Compute flags
	flags := FlagsDTO{
		Premium:   sub != nil,
		Reliable:  attendanceRate >= 0.9 && metricsDTO.TotalAppointments >= 5,
		Attention: metricsDTO.NoShow >= 2 || (metricsDTO.TotalAppointments >= 5 && attendanceRate < 0.7),
		AtRisk:    category == domainMetrics.CategoryAtRisk,
	}

	// 8. Derive booking policy
	policy := resolvePolicy(metricsDTO)

	return &ResponseDTO{
		Identity: IdentityDTO{
			ID:    client.ID,
			Name:  client.Name,
			Phone: client.Phone,
			Email: client.Email,
		},
		Category: CategoryDTO{
			Value:  string(category),
			Source: categorySource,
		},
		Metrics:      metricsDTO,
		Flags:        flags,
		Subscription: sub,
		Policy:       policy,
	}, nil
}

// resolvePolicy derives the operational booking policy for the client.
// Payment upfront is only required when there is concrete no-show evidence,
// not for inactivity or cancellations.
func resolvePolicy(metrics MetricsDTO) PolicyDTO {
	hasHighNoShowRate := metrics.TotalAppointments >= 5 &&
		float64(metrics.NoShow)/float64(metrics.TotalAppointments) >= 0.20
	if metrics.NoShow >= 2 || hasHighNoShowRate {
		return PolicyDTO{
			RequiresPaymentUpfront: true,
			Reason:                 "no_show",
		}
	}
	return PolicyDTO{RequiresPaymentUpfront: false}
}
