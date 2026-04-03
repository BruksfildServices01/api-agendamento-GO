package crm

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	domainMetrics "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
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
	// 1. Load client identity
	var client struct {
		ID    uint   `gorm:"column:id"`
		Name  string `gorm:"column:name"`
		Phone string `gorm:"column:phone"`
		Email string `gorm:"column:email"`
	}
	if err := q.db.WithContext(ctx).
		Table("clients").
		Select("id, name, phone, email").
		Where("id = ? AND barbershop_id = ?", clientID, barbershopID).
		First(&client).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrClientNotFound
		}
		return nil, err
	}

	// 2. Load metrics
	var m struct {
		TotalAppointments       int        `gorm:"column:total_appointments"`
		CompletedAppointments   int        `gorm:"column:completed_appointments"`
		CancelledAppointments   int        `gorm:"column:cancelled_appointments"`
		LateCancelledAppointments int      `gorm:"column:late_cancelled_appointments"`
		NoShowAppointments      int        `gorm:"column:no_show_appointments"`
		RescheduledAppointments int        `gorm:"column:rescheduled_appointments"`
		LateRescheduledAppointments int    `gorm:"column:late_rescheduled_appointments"`
		TotalSpent              int64      `gorm:"column:total_spent"`
		FirstAppointmentAt      *time.Time `gorm:"column:first_appointment_at"`
		LastAppointmentAt       *time.Time `gorm:"column:last_appointment_at"`
		LastCompletedAt         *time.Time `gorm:"column:last_completed_at"`
		Category                string     `gorm:"column:category"`
		CategorySource          string     `gorm:"column:category_source"`
		ManualCategoryExpiresAt *time.Time `gorm:"column:manual_category_expires_at"`
	}

	metricsFound := true
	if err := q.db.WithContext(ctx).
		Table("client_metrics").
		Where("client_id = ? AND barbershop_id = ?", clientID, barbershopID).
		First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			metricsFound = false
		} else {
			return nil, err
		}
	}

	// 3. Resolve category (apply classifier for auto, respect manual if not expired)
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

	// 4. Load active subscription
	var sub *SubscriptionDTO
	var subRow struct {
		PlanID       uint      `gorm:"column:plan_id"`
		PlanName     string    `gorm:"column:plan_name"`
		CutsUsed     int       `gorm:"column:cuts_used"`
		CutsIncluded int       `gorm:"column:cuts_included"`
		ValidUntil   time.Time `gorm:"column:valid_until"`
	}
	err := q.db.WithContext(ctx).Raw(`
		SELECT s.plan_id, p.name AS plan_name,
		       s.cuts_used_in_period AS cuts_used,
		       p.cuts_included,
		       s.current_period_end AS valid_until
		FROM subscriptions s
		JOIN plans p ON p.id = s.plan_id
		WHERE s.barbershop_id = ?
		  AND s.client_id = ?
		  AND s.status = 'active'
		LIMIT 1
	`, barbershopID, clientID).Scan(&subRow).Error
	if err == nil && subRow.PlanID != 0 {
		sub = &SubscriptionDTO{
			PlanID:       subRow.PlanID,
			PlanName:     subRow.PlanName,
			CutsUsed:     subRow.CutsUsed,
			CutsIncluded: subRow.CutsIncluded,
			ValidUntil:   subRow.ValidUntil,
		}
	}

	// 5. Compute metrics DTO
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

	// 6. Compute flags
	flags := FlagsDTO{
		Premium:   sub != nil,
		Reliable:  attendanceRate >= 0.9 && metricsDTO.TotalAppointments >= 5,
		Attention: metricsDTO.TotalAppointments > 0 && attendanceRate < 0.7,
		AtRisk:    category == domainMetrics.CategoryAtRisk,
	}

	// 7. Derive booking policy
	policy := resolvePolicy(category, flags)

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
func resolvePolicy(category domainMetrics.ClientCategory, flags FlagsDTO) PolicyDTO {
	switch {
	case category == domainMetrics.CategoryAtRisk:
		return PolicyDTO{
			RequiresPaymentUpfront: true,
			Reason:                 "at_risk_client",
		}
	case flags.Attention:
		return PolicyDTO{
			RequiresPaymentUpfront: true,
			Reason:                 "low_attendance_rate",
		}
	default:
		return PolicyDTO{RequiresPaymentUpfront: false}
	}
}
