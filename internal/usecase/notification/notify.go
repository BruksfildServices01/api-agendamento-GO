package notification

import (
	"context"
	"fmt"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/calendar"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/email"
)

type Notify struct {
	sender email.Sender
	ics    *calendar.ICSGenerator
}

func NewNotify(
	sender email.Sender,
	ics *calendar.ICSGenerator,
) *Notify {
	return &Notify{
		sender: sender,
		ics:    ics,
	}
}

func (n *Notify) Notify(
	ctx context.Context,
	ev domain.Event,
) error {

	var subject string
	var body string
	var cancelled bool

	switch ev.Type {
	case domain.EventPaymentConfirmed:
		subject = "Pagamento confirmado"
		body = fmt.Sprintf(
			"Olá %s,\n\nSeu pagamento foi confirmado.\n\n%s",
			ev.ClientName,
			ev.Title,
		)

	case domain.EventAppointmentCancelled:
		subject = "Agendamento cancelado"
		body = "Seu agendamento foi cancelado."
		cancelled = true

	case domain.EventAppointmentReminder:
		subject = "Lembrete de agendamento"
		body = "Este é um lembrete do seu agendamento."
	}

	icsFile, _ := n.ics.Generate(
		fmt.Sprintf("appointment-%d", ev.AppointmentID),
		ev.Title,
		ev.Description,
		ev.StartTime,
		ev.EndTime,
		ev.Timezone,
		cancelled,
	)

	return n.sender.Send(ctx, email.Message{
		To:      ev.ClientEmail,
		Subject: subject,
		Body:    body,
		Attachments: []email.Attachment{
			{
				Filename: "appointment.ics",
				Content:  icsFile,
			},
		},
	})
}
