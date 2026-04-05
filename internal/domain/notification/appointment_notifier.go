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
	ClientName      string
	ClientEmail     string
	BarbershopName  string
	BarbershopPhone string
	ServiceName     string
	StartTime       time.Time
	EndTime         time.Time
	Timezone        string
	TicketURL       string
}

type AppointmentCancelledInput struct {
	ClientName     string
	ClientEmail    string
	BarbershopName string
	StartTime      time.Time
	Timezone       string
}

type AppointmentRescheduledInput struct {
	ClientName      string
	ClientEmail     string
	BarbershopName  string
	BarbershopPhone string
	ServiceName     string
	OldStartTime    time.Time
	NewStartTime    time.Time
	NewEndTime      time.Time
	Timezone        string
	NewTicketURL    string
}
