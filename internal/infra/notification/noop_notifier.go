package notification

import (
	"context"

	domainNotification "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
)

type NoopNotifier struct{}

func NewNoopNotifier() *NoopNotifier {
	return &NoopNotifier{}
}

// Notify implementa domain/notification.Notifier
func (n *NoopNotifier) Notify(
	ctx context.Context,
	input domainNotification.PaymentConfirmedInput,
) error {
	// Sprint 6: não faz nada
	return nil
}
