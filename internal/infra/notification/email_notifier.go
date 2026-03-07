package notification

import (
	"context"
	"fmt"
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
