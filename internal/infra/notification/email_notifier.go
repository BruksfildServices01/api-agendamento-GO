package notification

import (
	"context"
	"log"
	"net/smtp"
	"strings"

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
	addr := cfg.SMTPHost + ":" + cfg.SMTPPort
	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
	from := cfg.EmailFrom

	log.Println("[EMAIL] notifier created, from:", from, "smtp:", addr)

	return &EmailNotifier{
		fromAddress: from,
		fromHeader:  "Corteon <" + from + ">",
		addr:        addr,
		auth:        auth,
	}
}

// ── Pagamento confirmado (Checkout Transparente / PIX) ───────────────────────

func (n *EmailNotifier) Notify(ctx context.Context, input domain.PaymentConfirmedInput) error {
	log.Println("[EMAIL] Notify (payment_confirmed) to:", input.ClientEmail)

	html, err := renderPaymentConfirmed(input)
	if err != nil {
		return err
	}

	ics := buildICS(input)
	return n.sendWithICS(input.ClientEmail, "Pagamento confirmado – Corteon", html, ics)
}

// ── Agendamento confirmado (sem pagamento) ───────────────────────────────────

func (n *EmailNotifier) NotifyConfirmed(ctx context.Context, input domain.AppointmentConfirmedInput) error {
	log.Println("[EMAIL] NotifyConfirmed to:", input.ClientEmail)

	html, err := renderAppointmentConfirmed(input)
	if err != nil {
		return err
	}

	ics := buildAppointmentICS(input)
	return n.sendWithICS(input.ClientEmail, "Agendamento confirmado – Corteon", html, ics)
}

// ── Agendamento cancelado ────────────────────────────────────────────────────

func (n *EmailNotifier) NotifyCancelled(ctx context.Context, input domain.AppointmentCancelledInput) error {
	log.Println("[EMAIL] NotifyCancelled to:", input.ClientEmail)

	html, err := renderAppointmentCancelled(input)
	if err != nil {
		return err
	}

	return n.sendHTML(input.ClientEmail, "Agendamento cancelado – Corteon", html)
}

// ── Agendamento remarcado ────────────────────────────────────────────────────

func (n *EmailNotifier) NotifyRescheduled(ctx context.Context, input domain.AppointmentRescheduledInput) error {
	log.Println("[EMAIL] NotifyRescheduled to:", input.ClientEmail)

	html, err := renderAppointmentRescheduled(input)
	if err != nil {
		return err
	}

	ics := buildRescheduledICS(input)
	return n.sendWithICS(input.ClientEmail, "Agendamento remarcado – Corteon", html, ics)
}

// ── helpers de envio ─────────────────────────────────────────────────────────

// sendHTML envia um e-mail somente com HTML (sem anexo .ics).
func (n *EmailNotifier) sendHTML(to, subject, html string) error {
	boundary := "BOUNDARY_ALT"

	var b strings.Builder
	b.WriteString("From: " + n.fromHeader + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=" + boundary + "\r\n\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString("Este e-mail requer um cliente com suporte a HTML.\r\n\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(html + "\r\n")

	b.WriteString("--" + boundary + "--\r\n")

	return smtp.SendMail(n.addr, n.auth, n.fromAddress, []string{to}, []byte(b.String()))
}

// sendWithICS envia um e-mail com HTML + anexo .ics.
func (n *EmailNotifier) sendWithICS(to, subject, html, ics string) error {
	mixed := "BOUNDARY_MIXED"
	alt := "BOUNDARY_ALT"

	var b strings.Builder
	b.WriteString("From: " + n.fromHeader + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/mixed; boundary=" + mixed + "\r\n\r\n")

	// HTML part
	b.WriteString("--" + mixed + "\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=" + alt + "\r\n\r\n")

	b.WriteString("--" + alt + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString("Este e-mail requer um cliente com suporte a HTML.\r\n\r\n")

	b.WriteString("--" + alt + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(html + "\r\n")

	b.WriteString("--" + alt + "--\r\n")

	// ICS attachment
	b.WriteString("--" + mixed + "\r\n")
	b.WriteString("Content-Type: text/calendar; charset=UTF-8; method=REQUEST\r\n")
	b.WriteString("Content-Disposition: attachment; filename=\"agendamento.ics\"\r\n\r\n")
	b.WriteString(ics + "\r\n")

	b.WriteString("--" + mixed + "--\r\n")

	return smtp.SendMail(n.addr, n.auth, n.fromAddress, []string{to}, []byte(b.String()))
}
