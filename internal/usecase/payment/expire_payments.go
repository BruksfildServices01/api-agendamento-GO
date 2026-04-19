package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domainAppointment "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type ExpirePayments struct {
	paymentRepo     domainPayment.Repository
	appointmentRepo domainAppointment.Repository
	audit           *audit.Dispatcher
}

func NewExpirePayments(
	paymentRepo domainPayment.Repository,
	appointmentRepo domainAppointment.Repository,
	audit *audit.Dispatcher,
) *ExpirePayments {
	return &ExpirePayments{
		paymentRepo:     paymentRepo,
		appointmentRepo: appointmentRepo,
		audit:           audit,
	}
}

func (uc *ExpirePayments) Execute(
	ctx context.Context,
	now time.Time,
	barbershopID uint,
) error {

	// Fast-path: skip transaction overhead when there is nothing to expire.
	// ListExpiredPending is a non-locking read; the authoritative lock happens
	// inside the transaction via ListExpiredPendingForUpdate below.
	pending, err := uc.paymentRepo.ListExpiredPending(ctx, barbershopID, now)
	if err != nil {
		return fmt.Errorf("expire job pre-check failed: %w", err)
	}
	if len(pending) == 0 {
		return nil
	}

	tx, err := uc.paymentRepo.BeginTx(ctx, barbershopID)
	if err != nil {
		return fmt.Errorf("expire job begin tx failed: %w", err)
	}
	defer tx.Rollback()

	payments, err := tx.ListExpiredPendingForUpdate(ctx, barbershopID, now)
	if err != nil {
		return fmt.Errorf("failed to lock expired payments: %w", err)
	}

	if len(payments) == 0 {
		return tx.Commit()
	}

	for _, p := range payments {
		currentStatus := domainPayment.Status(p.Status)

		if currentStatus != domainPayment.StatusPending {
			continue
		}
		if !currentStatus.CanTransitionTo(domainPayment.StatusExpired) {
			continue
		}

		if err := tx.MarkAsExpired(ctx, barbershopID, p); err != nil {
			return fmt.Errorf("failed to mark payment expired: %w", err)
		}

		if p.AppointmentID != nil {
			ap, err := tx.GetAppointmentForUpdate(ctx, barbershopID, *p.AppointmentID)
			if err != nil {
				return fmt.Errorf("failed to lock appointment: %w", err)
			}
			if ap != nil && ap.Status == models.AppointmentStatus(domainAppointment.StatusAwaitingPayment) {
				if err := domainAppointment.Cancel(ap, now); err == nil {
					if err := tx.UpdateAppointmentTx(ctx, ap); err != nil {
						return fmt.Errorf("failed to update appointment: %w", err)
					}
					uc.audit.Dispatch(audit.Event{
						BarbershopID: p.BarbershopID,
						Action:       "appointment_cancelled_by_payment_expiration",
						Entity:       "appointment",
						EntityID:     &ap.ID,
					})
				}
			}
		}

		if p.OrderID != nil {
			order, err := tx.GetOrderForUpdate(ctx, barbershopID, *p.OrderID)
			if err != nil {
				return fmt.Errorf("failed to lock order: %w", err)
			}

			if order != nil && order.Status == models.OrderStatusPending {
				order.Status = models.OrderStatusCancelled

				if err := tx.UpdateOrderTx(ctx, order); err != nil {
					return fmt.Errorf("failed to update order: %w", err)
				}

				uc.audit.Dispatch(audit.Event{
					BarbershopID: p.BarbershopID,
					Action:       "order_cancelled_by_payment_expiration",
					Entity:       "order",
					EntityID:     &order.ID,
				})
			}
		}

		uc.audit.Dispatch(audit.Event{
			BarbershopID: p.BarbershopID,
			Action:       "payment_expired",
			Entity:       "payment",
			EntityID:     &p.ID,
		})
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("expire job commit failed: %w", err)
	}

	return nil
}
