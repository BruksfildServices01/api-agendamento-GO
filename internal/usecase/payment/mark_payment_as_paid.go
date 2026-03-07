package payment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domainAppointment "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	domainNotification "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
	domainOrder "github.com/BruksfildServices01/barber-scheduler/internal/domain/order"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/idempotency"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

const pixPaidEvent = "pix_paid"

type MarkPaymentAsPaid struct {
	paymentRepo domainPayment.Repository
	audit       *audit.Dispatcher
	notifier    domainNotification.Notifier
	idem        idempotency.Store
}

func NewMarkPaymentAsPaid(
	paymentRepo domainPayment.Repository,
	audit *audit.Dispatcher,
	notifier domainNotification.Notifier,
	idem idempotency.Store,
) *MarkPaymentAsPaid {
	return &MarkPaymentAsPaid{
		paymentRepo: paymentRepo,
		audit:       audit,
		notifier:    notifier,
		idem:        idem,
	}
}

func (uc *MarkPaymentAsPaid) Execute(
	ctx context.Context,
	txid string,
) error {

	if txid == "" {
		return errors.New("txid is required")
	}

	// ==================================================
	// 1️⃣ Idempotência global (antes de qualquer coisa)
	// ==================================================
	idemKey := "pix:webhook:" + txid

	exists, err := uc.idem.Exists(ctx, idemKey)
	if err != nil {
		return fmt.Errorf("idempotency check failed: %w", err)
	}
	if exists {
		return nil
	}

	// ==================================================
	// 2️⃣ Resolver tenant pelo pagamento
	// ==================================================
	paymentBase, err := uc.paymentRepo.GetByTxIDGlobal(ctx, txid)
	if err != nil {
		return fmt.Errorf("failed to load payment: %w", err)
	}
	if paymentBase == nil {
		return fmt.Errorf("payment not found for txid: %s", txid)
	}

	barbershopID := paymentBase.BarbershopID

	// ==================================================
	// 3️⃣ BEGIN TX
	// ==================================================
	tx, err := uc.paymentRepo.BeginTx(ctx, barbershopID)
	if err != nil {
		return fmt.Errorf("begin tx failed: %w", err)
	}
	defer tx.Rollback()

	// ==================================================
	// 4️⃣ Lock payment
	// ==================================================
	payment, err := tx.GetByTxIDForUpdate(ctx, barbershopID, txid)
	if err != nil {
		return fmt.Errorf("failed to lock payment: %w", err)
	}
	if payment == nil {
		return fmt.Errorf("payment not found")
	}

	currentStatus := domainPayment.Status(payment.Status)

	if currentStatus.IsFinal() {
		return nil
	}

	if currentStatus != domainPayment.StatusPending {
		return fmt.Errorf(
			"invalid payment state transition: current=%s expected=pending",
			currentStatus,
		)
	}

	processed, err := tx.HasProcessedEvent(ctx, txid, pixPaidEvent)
	if err != nil {
		return fmt.Errorf("failed checking processed event: %w", err)
	}
	if processed {
		return nil
	}

	// ==================================================
	// 5️⃣ Atualizar payment
	// ==================================================
	now := time.Now().UTC()

	payment.Status = models.PaymentStatus(domainPayment.StatusPaid)
	payment.PaidAt = &now

	if err := tx.MarkAsPaid(ctx, barbershopID, payment); err != nil {
		return fmt.Errorf("failed to mark payment as paid: %w", err)
	}

	if err := tx.RegisterEvent(ctx, txid, pixPaidEvent); err != nil {
		return fmt.Errorf("failed to register pix event: %w", err)
	}

	// ==================================================
	// 6️⃣ Atualizar entidade associada (SE NECESSÁRIO)
	// ==================================================

	var ap *models.Appointment
	var order *models.Order

	// ---------- APPOINTMENT ----------
	if payment.AppointmentID != nil {

		ap, err = tx.GetAppointmentForUpdate(ctx, barbershopID, *payment.AppointmentID)
		if err != nil {
			return fmt.Errorf("failed to lock appointment: %w", err)
		}
		if ap == nil {
			return fmt.Errorf("appointment not found")
		}

		// 🔹 Caso 1: aguardando pagamento → vira scheduled
		if ap.Status == models.AppointmentStatus(domainAppointment.StatusAwaitingPayment) {

			ap.Status = models.AppointmentStatus(domainAppointment.StatusScheduled)

			if err := tx.UpdateAppointmentTx(ctx, ap); err != nil {
				return fmt.Errorf("failed to update appointment: %w", err)
			}
		}

		// 🔹 Caso 2: já estava scheduled (pay_later)
		// Não altera status, apenas mantém consistência
	}

	// ---------- ORDER ----------
	if payment.OrderID != nil {

		order, err = tx.GetOrderForUpdate(ctx, barbershopID, *payment.OrderID)
		if err != nil {
			return fmt.Errorf("failed to lock order: %w", err)
		}
		if order == nil {
			return fmt.Errorf("order not found")
		}

		if order.Status == models.OrderStatusPending {

			order.Status = models.OrderStatus(domainOrder.OrderStatusPaid)

			if err := tx.UpdateOrderTx(ctx, order); err != nil {
				return fmt.Errorf("failed to update order: %w", err)
			}
		}
	}

	// ==================================================
	// 7️⃣ COMMIT
	// ==================================================
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	if err := uc.idem.Save(ctx, idemKey); err != nil {
		return fmt.Errorf("failed to persist idempotency key: %w", err)
	}

	// ==================================================
	// 8️⃣ Auditoria (fora da transação)
	// ==================================================
	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		Action:       "payment_pix_confirmed",
		Entity:       "payment",
		EntityID:     &payment.ID,
	})

	if ap != nil {
		uc.audit.Dispatch(audit.Event{
			BarbershopID: barbershopID,
			Action:       "appointment_payment_confirmed",
			Entity:       "appointment",
			EntityID:     &ap.ID,
		})
	}

	if order != nil {
		uc.audit.Dispatch(audit.Event{
			BarbershopID: barbershopID,
			Action:       "order_payment_confirmed",
			Entity:       "order",
			EntityID:     &order.ID,
		})
	}

	return nil
}
