package notification

import (
	"fmt"
	"strings"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
)

func buildICS(input domain.PaymentConfirmedInput) string {

	startUTC := input.StartTime.UTC().Format("20060102T150405Z")
	endUTC := input.EndTime.UTC().Format("20060102T150405Z")

	uid := fmt.Sprintf(
		"%s-%d@corteon",
		strings.ReplaceAll(input.ClientEmail, "@", "_"),
		input.StartTime.Unix(),
	)

	var b strings.Builder

	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//CorteOn//Agendamento//PT-BR\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")

	b.WriteString("BEGIN:VEVENT\r\n")
	b.WriteString("UID:" + uid + "\r\n")
	b.WriteString("DTSTAMP:" + time.Now().UTC().Format("20060102T150405Z") + "\r\n")
	b.WriteString("DTSTART:" + startUTC + "\r\n")
	b.WriteString("DTEND:" + endUTC + "\r\n")
	b.WriteString("SUMMARY:Agendamento confirmado\r\n")
	b.WriteString("DESCRIPTION:Pagamento confirmado na barbearia " + input.BarbershopName + "\r\n")
	b.WriteString("END:VEVENT\r\n")

	b.WriteString("END:VCALENDAR\r\n")

	return b.String()
}
