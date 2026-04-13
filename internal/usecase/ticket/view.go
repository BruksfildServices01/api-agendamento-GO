package ticket

import (
	"context"
	"time"

	"gorm.io/gorm"

	domainTicket "github.com/BruksfildServices01/barber-scheduler/internal/domain/ticket"
)

type TicketViewDTO struct {
	AppointmentID   uint      `json:"appointment_id"`
	Status          string    `json:"status"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time"`
	ServiceName     string    `json:"service_name"`
	BarbershopName  string    `json:"barbershop_name"`
	BarbershopSlug  string    `json:"barbershop_slug"`
	BarbershopPhone string    `json:"barbershop_phone"`
	BarberName      string    `json:"barber_name"`
	ClientName      string    `json:"client_name"`
	ClientPhone     string    `json:"client_phone"`
	Token           string    `json:"token"`
	ExpiresAt       time.Time `json:"expires_at"`
	RescheduleCount int       `json:"reschedule_count"`
	CanCancel       bool      `json:"can_cancel"`
	CanReschedule   bool      `json:"can_reschedule"`
}

type ViewTicket struct {
	db *gorm.DB
}

func NewViewTicket(db *gorm.DB) *ViewTicket {
	return &ViewTicket{db: db}
}

func (uc *ViewTicket) Execute(ctx context.Context, token string) (*TicketViewDTO, error) {
	type row struct {
		AppointmentID   uint      `gorm:"column:appointment_id"`
		Status          string    `gorm:"column:status"`
		StartTime       time.Time `gorm:"column:start_time"`
		EndTime         time.Time `gorm:"column:end_time"`
		ServiceName     string    `gorm:"column:service_name"`
		BarbershopName  string    `gorm:"column:barbershop_name"`
		BarbershopSlug  string    `gorm:"column:barbershop_slug"`
		BarbershopPhone string    `gorm:"column:barbershop_phone"`
		BarberName      string    `gorm:"column:barber_name"`
		ClientName      string    `gorm:"column:client_name"`
		ClientPhone     string    `gorm:"column:client_phone"`
		Token           string    `gorm:"column:token"`
		ExpiresAt       time.Time `gorm:"column:expires_at"`
		RescheduleCount int       `gorm:"column:reschedule_count"`
	}

	var r row
	err := uc.db.WithContext(ctx).Raw(`
		SELECT
			a.id             AS appointment_id,
			a.status         AS status,
			a.start_time     AS start_time,
			a.end_time       AS end_time,
			bs.name          AS service_name,
			b.name           AS barbershop_name,
			b.slug           AS barbershop_slug,
			b.phone          AS barbershop_phone,
			u.name           AS barber_name,
			c.name           AS client_name,
			c.phone          AS client_phone,
			t.token          AS token,
			t.expires_at     AS expires_at,
			a.reschedule_count AS reschedule_count
		FROM appointment_tickets t
		JOIN appointments a         ON a.id = t.appointment_id
		JOIN barbershops b          ON b.id = t.barbershop_id
		LEFT JOIN barbershop_services bs ON bs.id = a.barber_product_id
		LEFT JOIN users u           ON u.id = a.barber_id
		LEFT JOIN clients c         ON c.id = a.client_id
		WHERE t.token = ?
	`, token).Scan(&r).Error

	if err != nil {
		return nil, err
	}

	if r.AppointmentID == 0 {
		return nil, domainTicket.ErrTokenExpired
	}

	now := time.Now().UTC()
	cutoff := now.Add(2 * time.Hour)

	canCancel := (r.Status == "scheduled" || r.Status == "awaiting_payment") && r.StartTime.After(cutoff)
	canReschedule := r.Status == "scheduled" && r.RescheduleCount < 1 && r.StartTime.After(cutoff)

	return &TicketViewDTO{
		AppointmentID:   r.AppointmentID,
		Status:          r.Status,
		StartTime:       r.StartTime,
		EndTime:         r.EndTime,
		ServiceName:     r.ServiceName,
		BarbershopName:  r.BarbershopName,
		BarbershopSlug:  r.BarbershopSlug,
		BarbershopPhone: r.BarbershopPhone,
		BarberName:      r.BarberName,
		ClientName:      r.ClientName,
		ClientPhone:     r.ClientPhone,
		Token:           r.Token,
		ExpiresAt:       r.ExpiresAt,
		RescheduleCount: r.RescheduleCount,
		CanCancel:       canCancel,
		CanReschedule:   canReschedule,
	}, nil
}
