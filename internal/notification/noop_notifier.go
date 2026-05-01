package notification

import (
	"context"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
)

// NoopNotifier implements both domain.Notifier and domain.AppointmentNotifier.
// All methods are no-ops — use it when email is disabled.
type NoopNotifier struct{}

func NewNoopNotifier() *NoopNotifier { return &NoopNotifier{} }

// --- domain.Notifier ---

func (n *NoopNotifier) Notify(_ context.Context, _ domain.PaymentConfirmedInput) error {
	return nil
}

// --- domain.AppointmentNotifier ---

func (n *NoopNotifier) NotifyConfirmed(_ context.Context, _ domain.AppointmentConfirmedInput) error {
	return nil
}

func (n *NoopNotifier) NotifyCancelled(_ context.Context, _ domain.AppointmentCancelledInput) error {
	return nil
}

func (n *NoopNotifier) NotifyRescheduled(_ context.Context, _ domain.AppointmentRescheduledInput) error {
	return nil
}

func (n *NoopNotifier) SendPasswordReset(_ context.Context, _, _ string) error {
	return nil
}
