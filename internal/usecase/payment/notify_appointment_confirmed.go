package payment

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"

	domainNotification "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
	domainTicket "github.com/BruksfildServices01/barber-scheduler/internal/domain/ticket"
)

type appointmentNotifyRow struct {
	ClientName      string    `gorm:"column:client_name"`
	ClientEmail     string    `gorm:"column:client_email"`
	BarbershopName  string    `gorm:"column:barbershop_name"`
	BarbershopPhone string    `gorm:"column:barbershop_phone"`
	ServiceName     string    `gorm:"column:service_name"`
	Timezone        string    `gorm:"column:timezone"`
	StartTime       time.Time `gorm:"column:start_time"`
	EndTime         time.Time `gorm:"column:end_time"`
}

// sendAppointmentConfirmedEmail queries the necessary data and fires the confirmation email.
// Errors are logged but never propagated — notification failures must not fail the payment flow.
func sendAppointmentConfirmedEmail(
	ctx context.Context,
	db *gorm.DB,
	apptNotifier domainNotification.AppointmentNotifier,
	ticketRepo domainTicket.Repository,
	appURL string,
	appointmentID uint,
) {
	var row appointmentNotifyRow
	err := db.WithContext(ctx).Raw(`
		SELECT
			c.name  AS client_name,
			c.email AS client_email,
			b.name  AS barbershop_name,
			b.phone AS barbershop_phone,
			bs.name AS service_name,
			b.timezone,
			a.start_time,
			a.end_time
		FROM appointments a
		JOIN clients             c  ON c.id  = a.client_id
		JOIN barbershops         b  ON b.id  = a.barbershop_id
		JOIN barbershop_services bs ON bs.id = a.barber_product_id
		WHERE a.id = ?
	`, appointmentID).Scan(&row).Error
	if err != nil {
		log.Printf("[NOTIFY] failed to query notification data for appointment %d: %v", appointmentID, err)
		return
	}
	if row.ClientEmail == "" {
		return
	}

	ticketURL := ""
	if ticketRepo != nil {
		if ticket, terr := ticketRepo.GetByAppointmentID(ctx, appointmentID); terr == nil && ticket != nil {
			ticketURL = appURL + "/ticket/" + ticket.Token
		}
	}

	_ = apptNotifier.NotifyConfirmed(ctx, domainNotification.AppointmentConfirmedInput{
		ClientName:      row.ClientName,
		ClientEmail:     row.ClientEmail,
		BarbershopName:  row.BarbershopName,
		BarbershopPhone: row.BarbershopPhone,
		ServiceName:     row.ServiceName,
		StartTime:       row.StartTime,
		EndTime:         row.EndTime,
		Timezone:        row.Timezone,
		TicketURL:       ticketURL,
	})
}
