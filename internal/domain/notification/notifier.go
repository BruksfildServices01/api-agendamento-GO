package notification

import "context"

type Notifier interface {
	Notify(
		ctx context.Context,
		input PaymentConfirmedInput,
	) error
}
