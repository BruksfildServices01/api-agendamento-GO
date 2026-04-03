package notification

import (
	"bytes"
	_ "embed"
	"html/template"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
)

//go:embed templates/payment_confirmed.html
var paymentConfirmedRaw string

var paymentConfirmedTmpl = template.Must(
	template.New("payment_confirmed").Parse(paymentConfirmedRaw),
)

type paymentConfirmedData struct {
	ClientName       string
	AppointmentDate  string
	BarbershopName   string
	BarbershopAddress string
	BarbershopPhone  string
}

func renderPaymentConfirmed(input domain.PaymentConfirmedInput) (string, error) {
	loc, err := time.LoadLocation(input.Timezone)
	if err != nil {
		loc = time.UTC
	}

	data := paymentConfirmedData{
		ClientName:        input.ClientName,
		AppointmentDate:   input.StartTime.In(loc).Format("02/01/2006 às 15:04"),
		BarbershopName:    input.BarbershopName,
		BarbershopAddress: input.BarbershopAddress,
		BarbershopPhone:   input.BarbershopPhone,
	}

	var buf bytes.Buffer
	if err := paymentConfirmedTmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
