package notification

import (
	"context"
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
)

type EmailNotifier struct {
	fromAddress string
	fromHeader  string
	addr        string
	auth        smtp.Auth
}

func NewEmailNotifier(cfg *config.Config) *EmailNotifier {
	addr := fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort)

	auth := smtp.PlainAuth(
		"",
		cfg.SMTPUser,
		cfg.SMTPPass,
		cfg.SMTPHost,
	)

	from := cfg.EmailFrom
	fromHeader := "CorteOn <" + from + ">"

	log.Println("[EMAIL] notifier created")
	log.Println("[EMAIL] from:", from)
	log.Println("[EMAIL] smtp:", addr)

	return &EmailNotifier{
		fromAddress: from,
		fromHeader:  fromHeader,
		addr:        addr,
		auth:        auth,
	}
}

func (n *EmailNotifier) Notify(
	ctx context.Context,
	input domain.PaymentConfirmedInput,
) error {

	log.Println("[EMAIL] sending to:", input.ClientEmail)

	subject := "Pagamento confirmado – CorteOn"

	// HTML já pronto
	html, err := renderPaymentConfirmed(input)
	if err != nil {
		return err
	}

	// MIME boundaries
	boundaryMixed := "BOUNDARY_MIXED"
	boundaryAlt := "BOUNDARY_ALT"

	var b strings.Builder

	// =======================
	// HEADERS
	// =======================
	b.WriteString("From: " + n.fromHeader + "\r\n")
	b.WriteString("To: " + input.ClientEmail + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/mixed; boundary=" + boundaryMixed + "\r\n\r\n")

	// =======================
	// multipart/alternative
	// =======================
	b.WriteString("--" + boundaryMixed + "\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=" + boundaryAlt + "\r\n\r\n")

	// text/plain fallback
	b.WriteString("--" + boundaryAlt + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString("Seu pagamento foi confirmado com sucesso.\r\n\r\n")

	// text/html
	b.WriteString("--" + boundaryAlt + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(html + "\r\n")

	b.WriteString("--" + boundaryAlt + "--\r\n")

	b.WriteString("--" + boundaryMixed + "--\r\n")

	// =======================
	// ICS attachment (sempre gerado aqui)
	// =======================
	ics := buildICS(input)

	b.WriteString("--" + boundaryMixed + "\r\n")
	b.WriteString("Content-Type: text/calendar; charset=UTF-8; method=REQUEST\r\n")
	b.WriteString("Content-Disposition: attachment; filename=\"agendamento.ics\"\r\n\r\n")
	b.WriteString(ics)
	b.WriteString("\r\n")

	// =======================
	// SMTP SEND
	// =======================
	return smtp.SendMail(
		n.addr,
		n.auth,
		n.fromAddress,
		[]string{input.ClientEmail},
		[]byte(b.String()),
	)
}

func (n *EmailNotifier) NotifyConfirmed(
	ctx context.Context,
	input domain.AppointmentConfirmedInput,
) error {
	log.Println("[EMAIL] NotifyConfirmed to:", input.ClientEmail)

	loc, err := time.LoadLocation(input.Timezone)
	if err != nil {
		loc = time.UTC
	}

	formattedTime := input.StartTime.In(loc).Format("02/01/2006 às 15:04")

	html := fmt.Sprintf(`<!DOCTYPE html>
<html><body>
<p>Olá, %s!</p>
<p>Seu agendamento na barbearia <strong>%s</strong> foi confirmado.</p>
<p><strong>Serviço:</strong> %s</p>
<p><strong>Data e hora:</strong> %s</p>
<p><strong>Telefone da barbearia:</strong> %s</p>
<p><a href="%s">Ver detalhes do agendamento</a></p>
</body></html>`,
		input.ClientName,
		input.BarbershopName,
		input.ServiceName,
		formattedTime,
		input.BarbershopPhone,
		input.TicketURL,
	)

	subject := "Agendamento confirmado – CorteOn"
	boundaryMixed := "BOUNDARY_MIXED_CONF"
	boundaryAlt := "BOUNDARY_ALT_CONF"

	var b strings.Builder

	b.WriteString("From: " + n.fromHeader + "\r\n")
	b.WriteString("To: " + input.ClientEmail + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/mixed; boundary=" + boundaryMixed + "\r\n\r\n")

	b.WriteString("--" + boundaryMixed + "\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=" + boundaryAlt + "\r\n\r\n")

	b.WriteString("--" + boundaryAlt + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString("Seu agendamento foi confirmado.\r\n\r\n")

	b.WriteString("--" + boundaryAlt + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(html + "\r\n")

	b.WriteString("--" + boundaryAlt + "--\r\n")
	b.WriteString("--" + boundaryMixed + "--\r\n")

	ics := buildAppointmentICS(input)

	b.WriteString("--" + boundaryMixed + "\r\n")
	b.WriteString("Content-Type: text/calendar; charset=UTF-8; method=REQUEST\r\n")
	b.WriteString("Content-Disposition: attachment; filename=\"agendamento.ics\"\r\n\r\n")
	b.WriteString(ics)
	b.WriteString("\r\n")

	return smtp.SendMail(
		n.addr,
		n.auth,
		n.fromAddress,
		[]string{input.ClientEmail},
		[]byte(b.String()),
	)
}

func (n *EmailNotifier) NotifyCancelled(
	ctx context.Context,
	input domain.AppointmentCancelledInput,
) error {
	log.Println("[EMAIL] NotifyCancelled to:", input.ClientEmail)

	loc, err := time.LoadLocation(input.Timezone)
	if err != nil {
		loc = time.UTC
	}

	formattedTime := input.StartTime.In(loc).Format("02/01/2006 às 15:04")

	html := fmt.Sprintf(`<!DOCTYPE html>
<html><body>
<p>Olá, %s!</p>
<p>Seu agendamento na barbearia <strong>%s</strong> marcado para <strong>%s</strong> foi cancelado.</p>
<p>Se quiser remarcar, entre em contato com a barbearia.</p>
</body></html>`,
		input.ClientName,
		input.BarbershopName,
		formattedTime,
	)

	subject := "Agendamento cancelado – CorteOn"
	boundary := "BOUNDARY_ALT_CANC"

	var b strings.Builder

	b.WriteString("From: " + n.fromHeader + "\r\n")
	b.WriteString("To: " + input.ClientEmail + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=" + boundary + "\r\n\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString("Seu agendamento foi cancelado.\r\n\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(html + "\r\n")

	b.WriteString("--" + boundary + "--\r\n")

	return smtp.SendMail(
		n.addr,
		n.auth,
		n.fromAddress,
		[]string{input.ClientEmail},
		[]byte(b.String()),
	)
}

func (n *EmailNotifier) NotifyRescheduled(
	ctx context.Context,
	input domain.AppointmentRescheduledInput,
) error {
	log.Println("[EMAIL] NotifyRescheduled to:", input.ClientEmail)

	loc, err := time.LoadLocation(input.Timezone)
	if err != nil {
		loc = time.UTC
	}

	formattedNewTime := input.NewStartTime.In(loc).Format("02/01/2006 às 15:04")

	html := fmt.Sprintf(`<!DOCTYPE html>
<html><body>
<p>Olá, %s!</p>
<p>Seu agendamento na barbearia <strong>%s</strong> foi remarcado.</p>
<p><strong>Serviço:</strong> %s</p>
<p><strong>Nova data e hora:</strong> %s</p>
<p><strong>Telefone da barbearia:</strong> %s</p>
<p><a href="%s">Ver detalhes do agendamento</a></p>
</body></html>`,
		input.ClientName,
		input.BarbershopName,
		input.ServiceName,
		formattedNewTime,
		input.BarbershopPhone,
		input.NewTicketURL,
	)

	subject := "Agendamento remarcado – CorteOn"
	boundaryMixed := "BOUNDARY_MIXED_RESCH"
	boundaryAlt := "BOUNDARY_ALT_RESCH"

	var b strings.Builder

	b.WriteString("From: " + n.fromHeader + "\r\n")
	b.WriteString("To: " + input.ClientEmail + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/mixed; boundary=" + boundaryMixed + "\r\n\r\n")

	b.WriteString("--" + boundaryMixed + "\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=" + boundaryAlt + "\r\n\r\n")

	b.WriteString("--" + boundaryAlt + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString("Seu agendamento foi remarcado.\r\n\r\n")

	b.WriteString("--" + boundaryAlt + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(html + "\r\n")

	b.WriteString("--" + boundaryAlt + "--\r\n")
	b.WriteString("--" + boundaryMixed + "--\r\n")

	ics := buildRescheduledICS(input)

	b.WriteString("--" + boundaryMixed + "\r\n")
	b.WriteString("Content-Type: text/calendar; charset=UTF-8; method=REQUEST\r\n")
	b.WriteString("Content-Disposition: attachment; filename=\"agendamento.ics\"\r\n\r\n")
	b.WriteString(ics)
	b.WriteString("\r\n")

	return smtp.SendMail(
		n.addr,
		n.auth,
		n.fromAddress,
		[]string{input.ClientEmail},
		[]byte(b.String()),
	)
}
