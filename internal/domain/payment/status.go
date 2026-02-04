package payment

type Status string

const (
	StatusPending Status = "pending"
	StatusPaid    Status = "paid"
	StatusExpired Status = "expired"
)

func (s Status) IsFinal() bool {
	return s == StatusPaid || s == StatusExpired
}
