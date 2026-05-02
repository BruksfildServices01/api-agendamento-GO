package notification

import (
	"context"
	"time"
)

type AppointmentNotifier interface {
	NotifyConfirmed(ctx context.Context, input AppointmentConfirmedInput) error
	NotifyCancelled(ctx context.Context, input AppointmentCancelledInput) error
	NotifyRescheduled(ctx context.Context, input AppointmentRescheduledInput) error
}

type AppointmentConfirmedInput struct {
	BarbershopID    uint   // necessário para o WhatsApp notifier identificar a instância
	ClientName      string
	ClientEmail     string
	ClientPhone     string // usado pelo WhatsApp notifier
	BarbershopName  string
	BarbershopPhone string
	BarbershopSlug  string // para link público no WhatsApp
	ServiceName     string
	StartTime       time.Time
	EndTime         time.Time
	Timezone        string
	TicketURL       string
}

type AppointmentCancelledInput struct {
	BarbershopID uint   // necessário para o WhatsApp notifier identificar a instância
	ClientName   string
	ClientEmail  string
	ClientPhone  string
	BarbershopName string
	BarbershopSlug string
	StartTime    time.Time
	Timezone     string
}

type AppointmentRescheduledInput struct {
	BarbershopID    uint   // necessário para o WhatsApp notifier identificar a instância
	ClientName      string
	ClientEmail     string
	ClientPhone     string
	BarbershopName  string
	BarbershopPhone string
	BarbershopSlug  string
	ServiceName     string
	OldStartTime    time.Time
	NewStartTime    time.Time
	NewEndTime      time.Time
	Timezone        string
	NewTicketURL    string
}
