package ticket

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domainNotification "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
	domainTicket "github.com/BruksfildServices01/barber-scheduler/internal/domain/ticket"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
)

var ErrRescheduleNotAllowed = errors.New("reschedule not allowed")
var ErrRescheduleWindowClosed = errors.New("reschedule window has passed")
var ErrTooSoon = errors.New("appointment is too soon to reschedule")
var ErrTimeConflict = errors.New("time conflict")
var ErrOutsideWorkingHours = errors.New("outside working hours")

type RescheduleViaTicket struct {
	db       *gorm.DB
	repo     domainTicket.Repository
	notifier domainNotification.AppointmentNotifier
	metrics  *ucMetrics.UpdateClientMetrics
	audit    *audit.Dispatcher
}

func NewRescheduleViaTicket(
	db *gorm.DB,
	repo domainTicket.Repository,
	notifier domainNotification.AppointmentNotifier,
	metrics *ucMetrics.UpdateClientMetrics,
	auditDispatcher *audit.Dispatcher,
) *RescheduleViaTicket {
	return &RescheduleViaTicket{
		db:       db,
		repo:     repo,
		notifier: notifier,
		metrics:  metrics,
		audit:    auditDispatcher,
	}
}

func (uc *RescheduleViaTicket) Execute(ctx context.Context, token, date, timeStr string) (string, error) {
	ticket, err := uc.repo.GetByToken(ctx, token)
	if err != nil {
		return "", err
	}

	if time.Now().UTC().After(ticket.ExpiresAt) {
		return "", domainTicket.ErrTokenExpired
	}

	type apptRow struct {
		ID              uint      `gorm:"column:id"`
		Status          string    `gorm:"column:status"`
		StartTime       time.Time `gorm:"column:start_time"`
		BarberID        uint      `gorm:"column:barber_id"`
		BarbershopID    uint      `gorm:"column:barbershop_id"`
		BarberProductID uint      `gorm:"column:barber_product_id"`
		RescheduleCount int       `gorm:"column:reschedule_count"`
		ClientID        uint      `gorm:"column:client_id"`
	}

	var appt apptRow
	err = uc.db.WithContext(ctx).
		Raw("SELECT id, status, start_time, barber_id, barbershop_id, barber_product_id, reschedule_count, client_id FROM appointments WHERE id = ?", ticket.AppointmentID).
		Scan(&appt).Error
	if err != nil {
		return "", err
	}
	if appt.ID == 0 {
		return "", ErrRescheduleNotAllowed
	}

	if appt.Status != "scheduled" || appt.RescheduleCount >= 1 {
		return "", ErrRescheduleNotAllowed
	}

	if !appt.StartTime.After(time.Now().UTC().Add(2 * time.Hour)) {
		return "", ErrRescheduleWindowClosed
	}

	type barbershopRow struct {
		Timezone          string `gorm:"column:timezone"`
		MinAdvanceMinutes int    `gorm:"column:min_advance_minutes"`
	}

	var bs barbershopRow
	err = uc.db.WithContext(ctx).
		Raw("SELECT timezone, min_advance_minutes FROM barbershops WHERE id = ?", appt.BarbershopID).
		Scan(&bs).Error
	if err != nil {
		return "", err
	}

	loc, err := time.LoadLocation(bs.Timezone)
	if err != nil {
		loc = time.UTC
	}

	minAdvance := bs.MinAdvanceMinutes
	if minAdvance == 0 {
		minAdvance = 120
	}

	newStart, err := time.ParseInLocation("2006-01-02 15:04", date+" "+timeStr, loc)
	if err != nil {
		return "", fmt.Errorf("invalid date or time: %w", err)
	}
	newStartUTC := newStart.UTC()

	if !newStartUTC.After(time.Now().UTC().Add(time.Duration(minAdvance) * time.Minute)) {
		return "", ErrTooSoon
	}

	type serviceRow struct {
		DurationMin int `gorm:"column:duration_min"`
	}

	var svc serviceRow
	err = uc.db.WithContext(ctx).
		Raw("SELECT duration_min FROM barbershop_services WHERE id = ?", appt.BarberProductID).
		Scan(&svc).Error
	if err != nil {
		return "", err
	}
	if svc.DurationMin == 0 {
		svc.DurationMin = 30
	}

	newEnd := newStartUTC.Add(time.Duration(svc.DurationMin) * time.Minute)

	weekday := int(newStart.Weekday())

	type whRow struct {
		StartTime  string `gorm:"column:start_time"`
		EndTime    string `gorm:"column:end_time"`
		LunchStart string `gorm:"column:lunch_start"`
		LunchEnd   string `gorm:"column:lunch_end"`
		Active     bool   `gorm:"column:active"`
	}

	var wh whRow
	err = uc.db.WithContext(ctx).
		Raw("SELECT start_time, end_time, lunch_start, lunch_end, active FROM working_hours WHERE barber_id = ? AND weekday = ? LIMIT 1", appt.BarberID, weekday).
		Scan(&wh).Error
	if err != nil {
		return "", err
	}

	if !wh.Active || wh.StartTime == "" {
		return "", ErrOutsideWorkingHours
	}

	datePrefix := newStart.Format("2006-01-02")
	parsedWHStart, err := time.ParseInLocation("2006-01-02 15:04", datePrefix+" "+wh.StartTime, loc)
	if err != nil {
		return "", ErrOutsideWorkingHours
	}
	parsedWHEnd, err := time.ParseInLocation("2006-01-02 15:04", datePrefix+" "+wh.EndTime, loc)
	if err != nil {
		return "", ErrOutsideWorkingHours
	}

	if newStart.Before(parsedWHStart) || newEnd.After(parsedWHEnd.UTC()) {
		return "", ErrOutsideWorkingHours
	}

	if wh.LunchStart != "" && wh.LunchEnd != "" {
		parsedLunchStart, err1 := time.ParseInLocation("2006-01-02 15:04", datePrefix+" "+wh.LunchStart, loc)
		parsedLunchEnd, err2 := time.ParseInLocation("2006-01-02 15:04", datePrefix+" "+wh.LunchEnd, loc)
		if err1 == nil && err2 == nil {
			lunchStartUTC := parsedLunchStart.UTC()
			lunchEndUTC := parsedLunchEnd.UTC()
			if newStartUTC.Before(lunchEndUTC) && newEnd.After(lunchStartUTC) {
				return "", ErrOutsideWorkingHours
			}
		}
	}

	var conflictCount int64
	err = uc.db.WithContext(ctx).
		Raw(`
			SELECT COUNT(*) FROM appointments
			WHERE barbershop_id = ?
			  AND barber_id = ?
			  AND status NOT IN ('cancelled', 'no_show')
			  AND id != ?
			  AND start_time < ?
			  AND end_time > ?
		`, appt.BarbershopID, appt.BarberID, appt.ID, newEnd, newStartUTC).
		Scan(&conflictCount).Error
	if err != nil {
		return "", err
	}
	if conflictCount > 0 {
		return "", ErrTimeConflict
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	newToken := hex.EncodeToString(raw)

	txErr := uc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"UPDATE appointments SET start_time = ?, end_time = ?, reschedule_count = reschedule_count + 1 WHERE id = ?",
			newStartUTC, newEnd, appt.ID,
		).Error; err != nil {
			return err
		}

		if err := tx.Exec(
			"UPDATE appointment_tickets SET expires_at = ?, token = ? WHERE id = ?",
			newStartUTC, newToken, ticket.ID,
		).Error; err != nil {
			return err
		}

		return nil
	})
	if txErr != nil {
		return "", txErr
	}

	// Auditoria
	if uc.audit != nil {
		uc.audit.Dispatch(audit.Event{
			BarbershopID: appt.BarbershopID,
			Action:       "ticket_reschedule",
			Entity:       "appointment",
			EntityID:     &appt.ID,
			Metadata: map[string]any{
				"old_start_time": appt.StartTime,
				"new_start_time": newStartUTC,
				"token":          newToken,
			},
		})
	}

	// Métrica: reagendamento tardio se estava a menos de 24h
	if uc.metrics != nil && appt.ClientID != 0 {
		eventType := ucMetrics.EventAppointmentRescheduled
		if appt.StartTime.Sub(time.Now().UTC()) < 24*time.Hour {
			eventType = ucMetrics.EventAppointmentLateRescheduled
		}
		_ = uc.metrics.Execute(ctx, ucMetrics.UpdateClientMetricsInput{
			BarbershopID: appt.BarbershopID,
			ClientID:     appt.ClientID,
			EventType:    eventType,
			OccurredAt:   time.Now().UTC(),
		})
	}

	if uc.notifier != nil {
		type notifyRow struct {
			ClientName      string `gorm:"column:client_name"`
			ClientEmail     string `gorm:"column:client_email"`
			BarbershopName  string `gorm:"column:barbershop_name"`
			BarbershopPhone string `gorm:"column:barbershop_phone"`
			ServiceName     string `gorm:"column:service_name"`
			Timezone        string `gorm:"column:timezone"`
		}
		var notifyData notifyRow
		queryErr := uc.db.WithContext(ctx).Raw(`
			SELECT c.name as client_name, c.email as client_email, b.name as barbershop_name, b.phone as barbershop_phone, bs.name as service_name, b.timezone
			FROM appointments a
			JOIN clients c ON c.id = a.client_id
			JOIN barbershops b ON b.id = a.barbershop_id
			JOIN barbershop_services bs ON bs.id = a.barber_product_id
			WHERE a.id = ?
		`, appt.ID).Scan(&notifyData).Error
		if queryErr != nil {
			log.Printf("[RescheduleViaTicket] failed to query notification data: %v", queryErr)
		} else if notifyData.ClientEmail != "" {
			_ = uc.notifier.NotifyRescheduled(ctx, domainNotification.AppointmentRescheduledInput{
				ClientName:      notifyData.ClientName,
				ClientEmail:     notifyData.ClientEmail,
				BarbershopName:  notifyData.BarbershopName,
				BarbershopPhone: notifyData.BarbershopPhone,
				ServiceName:     notifyData.ServiceName,
				OldStartTime:    appt.StartTime,
				NewStartTime:    newStartUTC,
				NewEndTime:      newEnd,
				Timezone:        notifyData.Timezone,
				NewTicketURL:    "/api/public/ticket/" + newToken,
			})
		}
	}

	return newToken, nil
}
