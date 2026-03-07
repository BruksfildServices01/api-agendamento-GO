package notification

import (
	"os"
	"strings"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
)

func renderPaymentConfirmed(input domain.PaymentConfirmedInput) (string, error) {

	raw, err := os.ReadFile(
		"internal/infra/notification/templates/payment_confirmed.html",
	)
	if err != nil {
		return "", err
	}

	html := string(raw)

	// timezone-safe
	loc, err := time.LoadLocation(input.Timezone)
	if err != nil {
		loc = time.UTC
	}

	start := input.StartTime.
		In(loc).
		Format("02/01/2006 às 15:04")

	html = strings.ReplaceAll(html, "{{ClientName}}", input.ClientName)
	html = strings.ReplaceAll(html, "{{AppointmentDate}}", start)
	html = strings.ReplaceAll(html, "{{BarbershopName}}", input.BarbershopName)
	html = strings.ReplaceAll(html, "{{BarbershopAddress}}", input.BarbershopAddress)
	html = strings.ReplaceAll(html, "{{BarbershopPhone}}", input.BarbershopPhone)

	return html, nil
}
