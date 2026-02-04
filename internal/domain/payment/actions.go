package payment

import (
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

func MarkAsPaid(p *models.Payment, now time.Time) error {
	if Status(p.Status).IsFinal() {
		return ErrInvalidState()
	}

	if Status(p.Status) != StatusPending {
		return ErrInvalidState()
	}

	p.Status = string(StatusPaid)
	p.PaidAt = &now
	return nil
}

func Expire(p *models.Payment, now time.Time) error {
	if Status(p.Status).IsFinal() {
		return ErrInvalidState()
	}

	if Status(p.Status) != StatusPending {
		return ErrInvalidState()
	}

	p.Status = string(StatusExpired)
	p.ExpiresAt = &now
	return nil
}
