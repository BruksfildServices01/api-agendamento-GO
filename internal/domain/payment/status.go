package payment

type Status string

const (
	StatusPending Status = "pending"
	StatusPaid    Status = "paid"
	StatusExpired Status = "expired"
)

// --------------------------------------------------
// Helpers
// --------------------------------------------------

func (s Status) IsFinal() bool {
	return s == StatusPaid || s == StatusExpired
}

// Transições válidas do estado atual → target
func (s Status) CanTransitionTo(target Status) bool {

	switch s {

	case StatusPending:
		return target == StatusPaid || target == StatusExpired

	case StatusPaid:
		// Pago nunca pode mudar
		return false

	case StatusExpired:
		// Expirado nunca pode virar pago
		return false
	}

	return false
}

// Validação forte de transição
func (s Status) MustTransitionTo(target Status) error {
	if !s.CanTransitionTo(target) {
		return ErrInvalidPaymentTransition(s, target)
	}
	return nil
}
