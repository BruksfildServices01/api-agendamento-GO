package email

import (
	"context"
	"fmt"
	"net/smtp"
	"os"
)

type SMTPSender struct {
	host string
	port string
	user string
	pass string
	from string
}

func NewSMTPSender() *SMTPSender {
	return &SMTPSender{
		host: os.Getenv("SMTP_HOST"),
		port: os.Getenv("SMTP_PORT"),
		user: os.Getenv("SMTP_USER"),
		pass: os.Getenv("SMTP_PASS"),
		from: os.Getenv("MAIL_FROM"),
	}
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error {

	addr := fmt.Sprintf("%s:%s", s.host, s.port)
	auth := smtp.PlainAuth("", s.user, s.pass, s.host)

	body := fmt.Sprintf(
		"To: %s\r\nSubject: %s\r\n\r\n%s",
		msg.To,
		msg.Subject,
		msg.Body,
	)

	return smtp.SendMail(
		addr,
		auth,
		s.from,
		[]string{msg.To},
		[]byte(body),
	)
}
