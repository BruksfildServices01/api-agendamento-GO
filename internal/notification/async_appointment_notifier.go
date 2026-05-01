package notification

import (
	"context"
	"log"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
)

// AsyncAppointmentNotifier wraps an AppointmentNotifier and fires all methods
// in a goroutine with a 30-second background timeout, always returning nil immediately.
type AsyncAppointmentNotifier struct {
	inner domain.AppointmentNotifier
}

func NewAsyncAppointmentNotifier(inner domain.AppointmentNotifier) *AsyncAppointmentNotifier {
	return &AsyncAppointmentNotifier{inner: inner}
}

func (a *AsyncAppointmentNotifier) NotifyConfirmed(_ context.Context, input domain.AppointmentConfirmedInput) error {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.inner.NotifyConfirmed(ctx, input); err != nil {
			log.Printf("[NOTIFY] NotifyConfirmed error for %s: %v", input.ClientEmail, err)
		}
	}()
	return nil
}

func (a *AsyncAppointmentNotifier) NotifyCancelled(_ context.Context, input domain.AppointmentCancelledInput) error {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.inner.NotifyCancelled(ctx, input); err != nil {
			log.Printf("[NOTIFY] NotifyCancelled error for %s: %v", input.ClientEmail, err)
		}
	}()
	return nil
}

func (a *AsyncAppointmentNotifier) NotifyRescheduled(_ context.Context, input domain.AppointmentRescheduledInput) error {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.inner.NotifyRescheduled(ctx, input); err != nil {
			log.Printf("[NOTIFY] NotifyRescheduled error for %s: %v", input.ClientEmail, err)
		}
	}()
	return nil
}
