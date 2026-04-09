package payment

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domainAppointment "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	domainNotification "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
	domainOrder "github.com/BruksfildServices01/barber-scheduler/internal/domain/order"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	domainTicket "github.com/BruksfildServices01/barber-scheduler/internal/domain/ticket"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/idempotency"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

const mpPaidEvent = "mp_paid"

// MarkMPPaymentAsPaid processa a confirmação de pagamento do Mercado Pago.
// Recebe o ID interno do pagamento (external_reference da preferência) e o
// mpPaymentID gerado pelo MP (usado como chave de idempotência).
type MarkMPPaymentAsPaid struct {
	paymentRepo  domainPayment.Repository
	audit        *audit.Dispatcher
	notifier     domainNotification.Notifier
	idem         idempotency.Store
	db           *gorm.DB
	apptNotifier domainNotification.AppointmentNotifier
	ticketRepo   domainTicket.Repository
	appURL       string
}

func NewMarkMPPaymentAsPaid(
	paymentRepo domainPayment.Repository,
	audit *audit.Dispatcher,
	notifier domainNotification.Notifier,
	idem idempotency.Store,
	db *gorm.DB,
	apptNotifier domainNotification.AppointmentNotifier,
	ticketRepo domainTicket.Repository,
	appURL string,
) *MarkMPPaymentAsPaid {
	return &MarkMPPaymentAsPaid{
		paymentRepo:  paymentRepo,
		audit:        audit,
		notifier:     notifier,
		idem:         idem,
		db:           db,
		apptNotifier: apptNotifier,
		ticketRepo:   ticketRepo,
		appURL:       appURL,
	}
}

// Execute processa a confirmação de um pagamento MP.
// externalReference é o campo external_reference da preferência = nosso payment ID.
// mpPaymentID é o ID do pagamento gerado pelo Mercado Pago (para idempotência).
func (uc *MarkMPPaymentAsPaid) Execute(
	ctx context.Context,
	externalReference string,
	mpPaymentID string,
) error {

	if externalReference == "" || mpPaymentID == "" {
		return fmt.Errorf("externalReference and mpPaymentID are required")
	}

	idemKey := "mp:webhook:" + mpPaymentID

	exists, err := uc.idem.Exists(ctx, idemKey)
	if err != nil {
		return fmt.Errorf("idempotency check failed: %w", err)
	}
	if exists {
		return nil
	}

	paymentID, err := strconv.ParseUint(externalReference, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid external_reference %q: %w", externalReference, err)
	}

	// Busca o payment pelo ID global (sem barbershopID — igual ao webhook PIX)
	paymentBase, err := uc.paymentRepo.GetByIDGlobal(ctx, uint(paymentID))
	if err != nil {
		return fmt.Errorf("failed to load payment: %w", err)
	}
	if paymentBase == nil {
		return fmt.Errorf("payment not found for external_reference: %s", externalReference)
	}

	barbershopID := paymentBase.BarbershopID

	// Se já está em estado final, o pagamento foi processado (aprovação imediata via
	// create_transparent_payment). O webhook chegou tarde — não há nada a fazer.
	if domainPayment.Status(paymentBase.Status).IsFinal() {
		return nil
	}

	tx, err := uc.paymentRepo.BeginTx(ctx, barbershopID)
	if err != nil {
		return fmt.Errorf("begin tx failed: %w", err)
	}
	defer tx.Rollback()

	// Usa o TxID (mp_pref:...) para o lock transacional
	if paymentBase.TxID == nil {
		return fmt.Errorf("payment %d has no TxID set", paymentID)
	}

	payment, err := tx.GetByTxIDForUpdate(ctx, barbershopID, *paymentBase.TxID)
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

	txid := *paymentBase.TxID

	processed, err := tx.HasProcessedEvent(ctx, txid, mpPaidEvent)
	if err != nil {
		return fmt.Errorf("failed checking processed event: %w", err)
	}
	if processed {
		return nil
	}

	now := time.Now().UTC()
	payment.Status = models.PaymentStatus(domainPayment.StatusPaid)
	payment.PaidAt = &now

	if err := tx.MarkAsPaid(ctx, barbershopID, payment); err != nil {
		return fmt.Errorf("failed to mark payment as paid: %w", err)
	}

	if err := tx.RegisterEvent(ctx, txid, mpPaidEvent); err != nil {
		return fmt.Errorf("failed to register mp event: %w", err)
	}

	var ap *models.Appointment
	var order *models.Order

	if payment.AppointmentID != nil {
		ap, err = tx.GetAppointmentForUpdate(ctx, barbershopID, *payment.AppointmentID)
		if err != nil {
			return fmt.Errorf("failed to lock appointment: %w", err)
		}
		if ap != nil && ap.Status == models.AppointmentStatus(domainAppointment.StatusAwaitingPayment) {
			ap.Status = models.AppointmentStatus(domainAppointment.StatusScheduled)
			if err := tx.UpdateAppointmentTx(ctx, ap); err != nil {
				return fmt.Errorf("failed to update appointment: %w", err)
			}
		}
	}

	// Marca pedido como pago — tanto via order_id (pedido standalone) quanto bundled_order_id (pedido embutido no pagamento do agendamento).
	bundledOrderID := payment.OrderID
	if bundledOrderID == nil {
		bundledOrderID = payment.BundledOrderID
	}
	if bundledOrderID != nil {
		order, err = tx.GetOrderForUpdate(ctx, barbershopID, *bundledOrderID)
		if err != nil {
			return fmt.Errorf("failed to lock order: %w", err)
		}
		if order != nil && order.Status == models.OrderStatusPending {
			items, err := tx.ListOrderItems(ctx, barbershopID, order.ID)
			if err != nil {
				return fmt.Errorf("failed to list order items: %w", err)
			}
			for _, it := range items {
				if err := tx.DecreaseProductStock(ctx, barbershopID, it.ProductID, it.Quantity); err != nil {
					return fmt.Errorf("failed to decrease stock: %w", err)
				}
			}
			order.Status = models.OrderStatus(domainOrder.OrderStatusPaid)
			if err := tx.UpdateOrderTx(ctx, order); err != nil {
				return fmt.Errorf("failed to update order: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	if err := uc.idem.Save(ctx, idemKey); err != nil {
		return fmt.Errorf("failed to persist idempotency key: %w", err)
	}

	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		Action:       "payment_mp_confirmed",
		Entity:       "payment",
		EntityID:     &payment.ID,
		Metadata: map[string]any{
			"mp_payment_id": mpPaymentID,
		},
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

	// Send appointment confirmation email after payment is confirmed.
	if ap != nil && uc.apptNotifier != nil && uc.db != nil {
		sendAppointmentConfirmedEmail(ctx, uc.db, uc.apptNotifier, uc.ticketRepo, uc.appURL, ap.ID)
	}

	return nil
}
