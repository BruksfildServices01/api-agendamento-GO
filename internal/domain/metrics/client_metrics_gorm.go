package metrics

import "time"

type CategorySource string

const (
	CategorySourceAuto   CategorySource = "auto"
	CategorySourceManual CategorySource = "manual"
)

type ClientMetrics struct {
	ClientID     uint
	BarbershopID uint

	TotalAppointments     int
	CompletedAppointments int
	CancelledAppointments int
	NoShowAppointments    int

	RescheduledAppointments     int
	LateCancelledAppointments   int
	LateRescheduledAppointments int

	TotalSpent int64

	FirstAppointmentAt *time.Time
	LastAppointmentAt  *time.Time
	LastCompletedAt    *time.Time
	LastCanceledAt     *time.Time

	LastNoShowAt          *time.Time
	LastLateCanceledAt    *time.Time
	LastLateRescheduledAt *time.Time

	Category                ClientCategory
	CategorySource          CategorySource
	ManualCategoryExpiresAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewClientMetrics(barbershopID, clientID uint) *ClientMetrics {
	return &ClientMetrics{
		BarbershopID:   barbershopID,
		ClientID:       clientID,
		Category:       CategoryNew,
		CategorySource: CategorySourceAuto,
	}
}

func (m *ClientMetrics) OnAppointmentCreated(at time.Time) {
	m.TotalAppointments++
	m.LastAppointmentAt = &at

	if m.FirstAppointmentAt == nil {
		m.FirstAppointmentAt = &at
	}
}

func (m *ClientMetrics) OnAppointmentCompleted(at time.Time, amount int64) {
	m.CompletedAppointments++
	m.TotalSpent += amount
	m.LastCompletedAt = &at
	m.LastAppointmentAt = &at

	m.recalculateCategory()
}

func (m *ClientMetrics) OnAppointmentCanceled(at time.Time) {
	m.CancelledAppointments++
	m.LastCanceledAt = &at

	m.recalculateCategory()
}

func (m *ClientMetrics) OnAppointmentNoShow(at time.Time) {
	m.NoShowAppointments++
	m.LastNoShowAt = &at
	m.LastAppointmentAt = &at

	m.recalculateCategory()
}

func (m *ClientMetrics) OnAppointmentRescheduled(at time.Time, late bool) {
	m.RescheduledAppointments++
	m.LastAppointmentAt = &at

	if late {
		m.LateRescheduledAppointments++
		m.LastLateRescheduledAt = &at
	}

	m.recalculateCategory()
}

func (m *ClientMetrics) OnLateCancellation(at time.Time) {
	m.LateCancelledAppointments++
	m.LastLateCanceledAt = &at

	m.recalculateCategory()
}

func (m *ClientMetrics) SetManualCategory(category ClientCategory, expiresAt *time.Time) {
	m.Category = category
	m.CategorySource = CategorySourceManual
	m.ManualCategoryExpiresAt = expiresAt
}

func (m *ClientMetrics) recalculateCategory() {
	if m.CategorySource == CategorySourceManual {
		// If the manual override has an expiration and it has passed, revert to auto.
		if m.ManualCategoryExpiresAt != nil && time.Now().UTC().After(*m.ManualCategoryExpiresAt) {
			m.CategorySource = CategorySourceAuto
			m.ManualCategoryExpiresAt = nil
		} else {
			return
		}
	}

	m.Category = Classify(m)
	m.CategorySource = CategorySourceAuto
}
