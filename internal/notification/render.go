package notification

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"net/url"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
)

//go:embed templates/payment_confirmed.html
var paymentConfirmedRaw string

//go:embed templates/appointment_confirmed.html
var appointmentConfirmedRaw string

//go:embed templates/appointment_cancelled.html
var appointmentCancelledRaw string

//go:embed templates/appointment_rescheduled.html
var appointmentRescheduledRaw string

var (
	paymentConfirmedTmpl      = template.Must(template.New("payment_confirmed").Parse(paymentConfirmedRaw))
	appointmentConfirmedTmpl  = template.Must(template.New("appointment_confirmed").Parse(appointmentConfirmedRaw))
	appointmentCancelledTmpl  = template.Must(template.New("appointment_cancelled").Parse(appointmentCancelledRaw))
	appointmentRescheduledTmpl = template.Must(template.New("appointment_rescheduled").Parse(appointmentRescheduledRaw))
)

// ── payment_confirmed ────────────────────────────────────────────────────────

type paymentConfirmedData struct {
	ClientName        string
	AppointmentDate   string
	BarbershopName    string
	BarbershopAddress string
	BarbershopPhone   string
}

func renderPaymentConfirmed(input domain.PaymentConfirmedInput) (string, error) {
	loc := loadLocation(input.Timezone)
	data := paymentConfirmedData{
		ClientName:        input.ClientName,
		AppointmentDate:   input.StartTime.In(loc).Format("02/01/2006 às 15:04"),
		BarbershopName:    input.BarbershopName,
		BarbershopAddress: input.BarbershopAddress,
		BarbershopPhone:   input.BarbershopPhone,
	}
	return execTemplate(paymentConfirmedTmpl, data)
}

// ── appointment_confirmed ────────────────────────────────────────────────────

type appointmentConfirmedData struct {
	ClientName         string
	ServiceName        string
	AppointmentDate    string
	BarbershopName     string
	BarbershopPhone    string
	TicketURL          string
	GoogleCalendarURL  string
}

func renderAppointmentConfirmed(input domain.AppointmentConfirmedInput) (string, error) {
	loc := loadLocation(input.Timezone)
	data := appointmentConfirmedData{
		ClientName:        input.ClientName,
		ServiceName:       input.ServiceName,
		AppointmentDate:   input.StartTime.In(loc).Format("02/01/2006 às 15:04"),
		BarbershopName:    input.BarbershopName,
		BarbershopPhone:   input.BarbershopPhone,
		TicketURL:         input.TicketURL,
		GoogleCalendarURL: buildGoogleCalendarURL(input.ServiceName, input.BarbershopName, input.StartTime, input.EndTime),
	}
	return execTemplate(appointmentConfirmedTmpl, data)
}

// ── appointment_cancelled ────────────────────────────────────────────────────

type appointmentCancelledData struct {
	ClientName      string
	AppointmentDate string
	BarbershopName  string
}

func renderAppointmentCancelled(input domain.AppointmentCancelledInput) (string, error) {
	loc := loadLocation(input.Timezone)
	data := appointmentCancelledData{
		ClientName:      input.ClientName,
		AppointmentDate: input.StartTime.In(loc).Format("02/01/2006 às 15:04"),
		BarbershopName:  input.BarbershopName,
	}
	return execTemplate(appointmentCancelledTmpl, data)
}

// ── appointment_rescheduled ──────────────────────────────────────────────────

type appointmentRescheduledData struct {
	ClientName         string
	ServiceName        string
	NewAppointmentDate string
	BarbershopName     string
	BarbershopPhone    string
	TicketURL          string
	GoogleCalendarURL  string
}

func renderAppointmentRescheduled(input domain.AppointmentRescheduledInput) (string, error) {
	loc := loadLocation(input.Timezone)
	data := appointmentRescheduledData{
		ClientName:         input.ClientName,
		ServiceName:        input.ServiceName,
		NewAppointmentDate: input.NewStartTime.In(loc).Format("02/01/2006 às 15:04"),
		BarbershopName:     input.BarbershopName,
		BarbershopPhone:    input.BarbershopPhone,
		TicketURL:          input.NewTicketURL,
		GoogleCalendarURL:  buildGoogleCalendarURL(input.ServiceName, input.BarbershopName, input.NewStartTime, input.NewEndTime),
	}
	return execTemplate(appointmentRescheduledTmpl, data)
}

// ── google calendar ──────────────────────────────────────────────────────────

func buildGoogleCalendarURL(serviceName, barbershopName string, start, end time.Time) string {
	fmtTime := func(t time.Time) string {
		return t.UTC().Format("20060102T150405Z")
	}
	title := fmt.Sprintf("%s — %s", serviceName, barbershopName)
	details := fmt.Sprintf("✂️ Serviço: %s\n💈 Barbearia: %s\n\nAgendado pelo Corteon.", serviceName, barbershopName)
	params := url.Values{}
	params.Set("action", "TEMPLATE")
	params.Set("text", title)
	params.Set("dates", fmtTime(start)+"/"+fmtTime(end))
	params.Set("details", details)
	return "https://calendar.google.com/calendar/render?" + params.Encode()
}

// ── helpers ──────────────────────────────────────────────────────────────────

func loadLocation(tz string) *time.Location {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

func execTemplate(tmpl *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
