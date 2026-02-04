package payment

import (
	"context"
	"log"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domainAppointment "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	domainNotification "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
)

const pixPaidEvent = "pix_paid"

type MarkPaymentAsPaid struct {
	paymentRepo     domainPayment.Repository
	appointmentRepo domainAppointment.JobRepository
	audit           *audit.Dispatcher
	notifier        domainNotification.Notifier
}

func NewMarkPaymentAsPaid(
	paymentRepo domainPayment.Repository,
	appointmentRepo domainAppointment.JobRepository,
	audit *audit.Dispatcher,
	notifier domainNotification.Notifier,
) *MarkPaymentAsPaid {
	return &MarkPaymentAsPaid{
		paymentRepo:     paymentRepo,
		appointmentRepo: appointmentRepo,
		audit:           audit,
		notifier:        notifier,
	}
}

func (uc *MarkPaymentAsPaid) Execute(
	ctx context.Context,
	txid string,
) error {

	// ==================================================
	// 1️⃣ BEGIN TX
	// ==================================================
	tx, err := uc.paymentRepo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// ==================================================
	// 2️⃣ Lock pessimista por TXID
	// ==================================================
	payment, err := tx.GetByTxIDForUpdate(ctx, txid)
	if err != nil || payment == nil {
		return nil
	}

	// ==================================================
	// 3️⃣ Idempotência por status
	// ==================================================
	if payment.Status != string(domainPayment.StatusPending) {
		return nil
	}

	// ==================================================
	// 4️⃣ Idempotência por evento
	// ==================================================
	processed, err := tx.HasProcessedEvent(ctx, txid, pixPaidEvent)
	if err != nil {
		return err
	}
	if processed {
		return nil
	}

	// ==================================================
	// 5️⃣ Marca payment como PAID
	// ==================================================
	now := timezone.Now()

	payment.Status = string(domainPayment.StatusPaid)
	payment.PaidAt = &now

	if err := tx.MarkAsPaid(ctx, payment); err != nil {
		return err
	}

	if err := tx.RegisterEvent(ctx, txid, pixPaidEvent); err != nil {
		return err
	}

	// ==================================================
	// 6️⃣ Atualiza appointment
	// ==================================================
	ap, err := uc.appointmentRepo.GetAppointmentByID(
		ctx,
		payment.AppointmentID,
	)
	if err != nil || ap == nil {
		return err
	}

	if ap.Status == string(domainAppointment.StatusAwaitingPayment) {
		ap.Status = string(domainAppointment.StatusScheduled)

		if err := uc.appointmentRepo.UpdateAppointment(ctx, ap); err != nil {
			return err
		}
	}

	// ==================================================
	// 7️⃣ COMMIT
	// ==================================================
	if err := tx.Commit(); err != nil {
		return err
	}

	// ==================================================
	// 8️⃣ Auditoria (fora da TX)
	// ==================================================
	uc.audit.Dispatch(audit.Event{
		BarbershopID: payment.BarbershopID,
		Action:       "payment_pix_confirmed",
		Entity:       "payment",
		EntityID:     &payment.ID,
	})

	uc.audit.Dispatch(audit.Event{
		BarbershopID: ap.BarbershopID,
		Action:       "appointment_payment_confirmed",
		Entity:       "appointment",
		EntityID:     &ap.ID,
	})

	// ==================================================
	// 9️⃣ Notificação (BEST EFFORT)
	// ==================================================
	// ⚠️ Se falhar, NÃO quebra o fluxo
	log.Println("[EMAIL] will send to:", ap.Client.Email)

	input := domainNotification.PaymentConfirmedInput{
		PaymentID: payment.ID,

		// -------- Barbearia --------
		BarbershopName:    ap.Barbershop.Name,
		BarbershopSlug:    ap.Barbershop.Slug,
		BarbershopAddress: ap.Barbershop.Address,
		BarbershopPhone:   ap.Barbershop.Phone,

		// -------- Cliente --------
		ClientName:  ap.Client.Name,
		ClientEmail: ap.Client.Email,

		// -------- Serviço --------
		ServiceName: ap.BarberProduct.Name,

		// -------- Horário --------
		StartTime: ap.StartTime,
		EndTime:   ap.EndTime,
		Timezone:  ap.Barbershop.Timezone,
	}

	// BEST EFFORT: erro aqui não quebra o fluxo
	_ = uc.notifier.Notify(ctx, input)

	return nil
}
