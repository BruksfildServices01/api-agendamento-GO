package payment

import (
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

func MarkAsPaid(p *models.Payment, now time.Time) error {

	current := Status(p.Status)

	if err := current.MustTransitionTo(StatusPaid); err != nil {
		return err
	}

	p.Status = models.PaymentStatus(StatusPaid)
	p.PaidAt = &now

	return nil
}

func Expire(p *models.Payment, now time.Time) error {

	current := Status(p.Status)

	if err := current.MustTransitionTo(StatusExpired); err != nil {
		return err
	}

	p.Status = models.PaymentStatus(StatusExpired)

	return nil
}
