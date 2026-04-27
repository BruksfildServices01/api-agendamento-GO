package ticket

import (
	"context"
	"errors"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domainNotification "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
	domainTicket "github.com/BruksfildServices01/barber-scheduler/internal/domain/ticket"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
)

var ErrCannotCancel = errors.New("appointment cannot be cancelled")
var ErrCancellationWindowClosed = errors.New("cancellation window has passed")

// lateCancelWindow: cancelamentos dentro deste prazo antes do agendamento
// são considerados tardios e geram penalidade na métrica do cliente.
const lateCancelWindow = 24 * time.Hour

// minCancelWindow: janela mínima para permitir cancelamento (2h antes).
const minCancelWindow = 2 * time.Hour

type CancelViaTicket struct {
	db       *gorm.DB
	repo     domainTicket.Repository
	notifier domainNotification.AppointmentNotifier
	metrics  *ucMetrics.UpdateClientMetrics
	audit    *audit.Dispatcher
}

func NewCancelViaTicket(
	db *gorm.DB,
	repo domainTicket.Repository,
	notifier domainNotification.AppointmentNotifier,
	metrics *ucMetrics.UpdateClientMetrics,
	auditDispatcher *audit.Dispatcher,
) *CancelViaTicket {
	return &CancelViaTicket{
		db:       db,
		repo:     repo,
		notifier: notifier,
		metrics:  metrics,
		audit:    auditDispatcher,
	}
}

func (uc *CancelViaTicket) Execute(ctx context.Context, token string) error {
	ticket, err := uc.repo.GetByToken(ctx, token)
	if err != nil {
		return err
	}

	type apptRow struct {
		ID                    uint      `gorm:"column:id"`
		Status                string    `gorm:"column:status"`
		StartTime             time.Time `gorm:"column:start_time"`
		BarberID              uint      `gorm:"column:barber_id"`
		BarbershopID          uint      `gorm:"column:barbershop_id"`
		ClientID              *uint     `gorm:"column:client_id"`
		ReservedSubscriptionCut bool    `gorm:"column:reserved_subscription_cut"`
		SubscriptionID        *uint     `gorm:"column:subscription_id"`
	}

	var appt apptRow
	err = uc.db.WithContext(ctx).
		Raw(`SELECT id, status, start_time, barber_id, barbershop_id, client_id,
		     reserved_subscription_cut, subscription_id
		     FROM appointments WHERE id = ?`, ticket.AppointmentID).
		Scan(&appt).Error
	if err != nil {
		return err
	}
	if appt.ID == 0 {
		return ErrCannotCancel
	}

	now := time.Now().UTC()
	if !appt.StartTime.After(now.Add(minCancelWindow)) {
		return ErrCancellationWindowClosed
	}

	// Cancela de forma atômica: o UPDATE verifica o status diretamente no banco,
	// eliminando a race condition de TOCTOU entre a leitura e a escrita.
	res := uc.db.WithContext(ctx).Exec(
		`UPDATE appointments
		 SET status = 'cancelled', cancelled_at = NOW()
		 WHERE id = ? AND barbershop_id = ? AND status IN ('scheduled', 'awaiting_payment')`,
		appt.ID, appt.BarbershopID,
	)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrCannotCancel
	}

	// Libera reserva de assinatura se existia (best-effort).
	if appt.ReservedSubscriptionCut && appt.ClientID != nil {
		releaseRes := uc.db.WithContext(ctx).Exec(`
			UPDATE subscriptions
			SET cuts_reserved_in_period = cuts_reserved_in_period - 1
			WHERE barbershop_id = ? AND client_id = ? AND status = 'active'
			  AND current_period_start <= NOW() AND current_period_end > NOW()
			  AND cuts_reserved_in_period > 0
		`, appt.BarbershopID, *appt.ClientID)
		if releaseRes.Error != nil {
			log.Printf("[CancelViaTicket] release subscription cut failed client=%d: %v",
				*appt.ClientID, releaseRes.Error)
		}
	}

	// Auditoria
	if uc.audit != nil {
		uc.audit.Dispatch(audit.Event{
			BarbershopID: appt.BarbershopID,
			Action:       "ticket_cancel",
			Entity:       "appointment",
			EntityID:     &appt.ID,
			Metadata: map[string]any{
				"token":      token,
				"start_time": appt.StartTime,
			},
		})
	}

	// Métrica do cliente: penalidade se cancelamento tardio (< 24h)
	if uc.metrics != nil && appt.ClientID != nil {
		eventType := ucMetrics.EventAppointmentCanceled
		if appt.StartTime.Sub(now) < lateCancelWindow {
			eventType = ucMetrics.EventAppointmentLateCanceled
		}
		_ = uc.metrics.Execute(ctx, ucMetrics.UpdateClientMetricsInput{
			BarbershopID: appt.BarbershopID,
			ClientID:     *appt.ClientID,
			EventType:    eventType,
			OccurredAt:   now,
		})
	}

	// Notificação
	if uc.notifier != nil {
		type notifyRow struct {
			ClientName     string `gorm:"column:client_name"`
			ClientEmail    string `gorm:"column:client_email"`
			ClientPhone    string `gorm:"column:client_phone"`
			BarbershopName string `gorm:"column:barbershop_name"`
			BarbershopSlug string `gorm:"column:barbershop_slug"`
			Timezone       string `gorm:"column:timezone"`
		}
		var notifyData notifyRow
		queryErr := uc.db.WithContext(ctx).Raw(`
			SELECT c.name  AS client_name,
			       c.email AS client_email,
			       c.phone AS client_phone,
			       b.name  AS barbershop_name,
			       b.slug  AS barbershop_slug,
			       b.timezone
			FROM appointments a
			JOIN clients     c ON c.id = a.client_id
			JOIN barbershops b ON b.id = a.barbershop_id
			WHERE a.id = ?
		`, appt.ID).Scan(&notifyData).Error
		if queryErr != nil {
			log.Printf("[CancelViaTicket] failed to query notification data: %v", queryErr)
		} else if notifyData.ClientEmail != "" || notifyData.ClientPhone != "" {
			_ = uc.notifier.NotifyCancelled(ctx, domainNotification.AppointmentCancelledInput{
				ClientName:     notifyData.ClientName,
				ClientEmail:    notifyData.ClientEmail,
				ClientPhone:    notifyData.ClientPhone,
				BarbershopName: notifyData.BarbershopName,
				BarbershopSlug: notifyData.BarbershopSlug,
				StartTime:      appt.StartTime,
				Timezone:       notifyData.Timezone,
			})
		}
	}

	return nil
}
