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
	ClientPhone     string    `gorm:"column:client_phone"`
	BarbershopName  string    `gorm:"column:barbershop_name"`
	BarbershopPhone string    `gorm:"column:barbershop_phone"`
	BarbershopSlug  string    `gorm:"column:barbershop_slug"`
	ServiceName     string    `gorm:"column:service_name"`
	Timezone        string    `gorm:"column:timezone"`
	StartTime       time.Time `gorm:"column:start_time"`
	EndTime         time.Time `gorm:"column:end_time"`
}

// sendAppointmentConfirmedNotification dispara a notificação de confirmação.
// Suporta email e WhatsApp — o notifier ativo decide o canal.
// Nunca propaga erros: falhas de notificação não devem afetar o fluxo de pagamento.
func sendAppointmentConfirmedNotification(
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
			c.phone AS client_phone,
			b.name  AS barbershop_name,
			b.phone AS barbershop_phone,
			b.slug  AS barbershop_slug,
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
	// Requer pelo menos email ou telefone para notificar
	if row.ClientEmail == "" && row.ClientPhone == "" {
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
		ClientPhone:     row.ClientPhone,
		BarbershopName:  row.BarbershopName,
		BarbershopPhone: row.BarbershopPhone,
		BarbershopSlug:  row.BarbershopSlug,
		ServiceName:     row.ServiceName,
		StartTime:       row.StartTime,
		EndTime:         row.EndTime,
		Timezone:        row.Timezone,
		TicketURL:       ticketURL,
	})
}
