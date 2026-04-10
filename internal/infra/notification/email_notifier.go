package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
)

type EmailNotifier struct {
	fromAddress string
	fromName    string

	// Brevo HTTP API (preferido)
	brevoAPIKey string

	// SMTP (fallback quando brevoAPIKey está vazio)
	smtpAddr string
	smtpAuth smtp.Auth
}

func NewEmailNotifier(cfg *config.Config) *EmailNotifier {
	n := &EmailNotifier{
		fromAddress: cfg.EmailFrom,
		fromName:    "Corteon",
		brevoAPIKey: cfg.BrevoAPIKey,
	}

	if cfg.BrevoAPIKey != "" {
		log.Println("[EMAIL] notifier created (Brevo HTTP API), from:", cfg.EmailFrom)
	} else {
		addr := cfg.SMTPHost + ":" + cfg.SMTPPort
		n.smtpAddr = addr
		n.smtpAuth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
		log.Println("[EMAIL] notifier created (SMTP), from:", cfg.EmailFrom, "smtp:", addr)
	}

	return n
}

// ── Pagamento confirmado (Checkout Transparente / PIX) ───────────────────────

func (n *EmailNotifier) Notify(ctx context.Context, input domain.PaymentConfirmedInput) error {
	log.Println("[EMAIL] Notify (payment_confirmed) to:", input.ClientEmail)

	html, err := renderPaymentConfirmed(input)
	if err != nil {
		log.Printf("[EMAIL] Notify render error: %v", err)
		return err
	}

	ics := buildICS(input)
	err = n.send(ctx, input.ClientEmail, "Pagamento confirmado – Corteon", html, ics)
	if err != nil {
		log.Printf("[EMAIL] Notify send error to=%s: %v", input.ClientEmail, err)
	}
	return err
}

// ── Agendamento confirmado (sem pagamento) ───────────────────────────────────

func (n *EmailNotifier) NotifyConfirmed(ctx context.Context, input domain.AppointmentConfirmedInput) error {
	log.Println("[EMAIL] NotifyConfirmed to:", input.ClientEmail)

	html, err := renderAppointmentConfirmed(input)
	if err != nil {
		log.Printf("[EMAIL] NotifyConfirmed render error: %v", err)
		return err
	}

	ics := buildAppointmentICS(input)
	err = n.send(ctx, input.ClientEmail, "Agendamento confirmado – Corteon", html, ics)
	if err != nil {
		log.Printf("[EMAIL] NotifyConfirmed send error to=%s: %v", input.ClientEmail, err)
	}
	return err
}

// ── Agendamento cancelado ────────────────────────────────────────────────────

func (n *EmailNotifier) NotifyCancelled(ctx context.Context, input domain.AppointmentCancelledInput) error {
	log.Println("[EMAIL] NotifyCancelled to:", input.ClientEmail)

	html, err := renderAppointmentCancelled(input)
	if err != nil {
		log.Printf("[EMAIL] NotifyCancelled render error: %v", err)
		return err
	}

	err = n.send(ctx, input.ClientEmail, "Agendamento cancelado – Corteon", html, "")
	if err != nil {
		log.Printf("[EMAIL] NotifyCancelled send error to=%s: %v", input.ClientEmail, err)
	}
	return err
}

// ── Agendamento remarcado ────────────────────────────────────────────────────

func (n *EmailNotifier) NotifyRescheduled(ctx context.Context, input domain.AppointmentRescheduledInput) error {
	log.Println("[EMAIL] NotifyRescheduled to:", input.ClientEmail)

	html, err := renderAppointmentRescheduled(input)
	if err != nil {
		log.Printf("[EMAIL] NotifyRescheduled render error: %v", err)
		return err
	}

	ics := buildRescheduledICS(input)
	err = n.send(ctx, input.ClientEmail, "Agendamento remarcado – Corteon", html, ics)
	if err != nil {
		log.Printf("[EMAIL] NotifyRescheduled send error to=%s: %v", input.ClientEmail, err)
	}
	return err
}

// ── dispatcher central ───────────────────────────────────────────────────────

func (n *EmailNotifier) send(ctx context.Context, to, subject, html, ics string) error {
	if n.brevoAPIKey != "" {
		return n.sendViaBrevoAPI(ctx, to, subject, html, ics)
	}
	if ics != "" {
		return n.sendWithICS(to, subject, html, ics)
	}
	return n.sendHTML(to, subject, html)
}

// ── Brevo HTTP API ───────────────────────────────────────────────────────────

type brevoEmailRequest struct {
	Sender     brevoContact   `json:"sender"`
	To         []brevoContact `json:"to"`
	Subject    string         `json:"subject"`
	HTMLContent string        `json:"htmlContent"`
}

type brevoContact struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
}

func (n *EmailNotifier) sendViaBrevoAPI(ctx context.Context, to, subject, html, _ string) error {
	payload := brevoEmailRequest{
		Sender:      brevoContact{Name: n.fromName, Email: n.fromAddress},
		To:          []brevoContact{{Email: to}},
		Subject:     subject,
		HTMLContent: html,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("brevo marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.brevo.com/v3/smtp/email", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("brevo request: %w", err)
	}
	req.Header.Set("api-key", n.brevoAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("brevo http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var errBody bytes.Buffer
		errBody.ReadFrom(resp.Body)
		return fmt.Errorf("brevo api status=%d body=%s", resp.StatusCode, errBody.String())
	}

	log.Printf("[EMAIL] Brevo API sent to=%s status=%d", to, resp.StatusCode)
	return nil
}

// ── SMTP helpers (fallback) ──────────────────────────────────────────────────

func (n *EmailNotifier) sendHTML(to, subject, html string) error {
	boundary := "BOUNDARY_ALT"

	var b strings.Builder
	b.WriteString("From: " + n.fromName + " <" + n.fromAddress + ">\r\n")
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

	return smtp.SendMail(n.smtpAddr, n.smtpAuth, n.fromAddress, []string{to}, []byte(b.String()))
}

func (n *EmailNotifier) sendWithICS(to, subject, html, ics string) error {
	mixed := "BOUNDARY_MIXED"
	alt := "BOUNDARY_ALT"

	var b strings.Builder
	b.WriteString("From: " + n.fromName + " <" + n.fromAddress + ">\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/mixed; boundary=" + mixed + "\r\n\r\n")

	b.WriteString("--" + mixed + "\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=" + alt + "\r\n\r\n")

	b.WriteString("--" + alt + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString("Este e-mail requer um cliente com suporte a HTML.\r\n\r\n")

	b.WriteString("--" + alt + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(html + "\r\n")

	b.WriteString("--" + alt + "--\r\n")

	b.WriteString("--" + mixed + "\r\n")
	b.WriteString("Content-Type: text/calendar; charset=UTF-8; method=REQUEST\r\n")
	b.WriteString("Content-Disposition: attachment; filename=\"agendamento.ics\"\r\n\r\n")
	b.WriteString(ics + "\r\n")

	b.WriteString("--" + mixed + "--\r\n")

	return smtp.SendMail(n.smtpAddr, n.smtpAuth, n.fromAddress, []string{to}, []byte(b.String()))
}
