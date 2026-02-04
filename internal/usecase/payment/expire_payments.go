package payment

import (
	"context"
	"log"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domainAppointment "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
)

// ExpirePayments é o use case responsável por
// expirar pagamentos pendentes vencidos e
// cancelar agendamentos que dependiam deles.
type ExpirePayments struct {
	paymentRepo     domainPayment.Repository
	appointmentRepo domainAppointment.JobRepository
	audit           *audit.Dispatcher
}

func NewExpirePayments(
	paymentRepo domainPayment.Repository,
	appointmentRepo domainAppointment.JobRepository,
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
) error {

	// --------------------------------------------------
	// 1️⃣ Buscar payments pendentes já expirados
	// --------------------------------------------------
	payments, err := uc.paymentRepo.ListExpiredPending(ctx, now)
	if err != nil {
		return err
	}

	// --------------------------------------------------
	// 2️⃣ Processar cada payment
	// --------------------------------------------------
	for _, p := range payments {

		log.Println(
			"[ExpirePayments]",
			"payment_id:", p.ID,
			"expires_at:", p.ExpiresAt,
			"now:", now,
			"status:", p.Status,
		)

		// Defesa extra
		if p.Status != string(domainPayment.StatusPending) {
			continue
		}

		// --------------------------------------------------
		// 2.1️⃣ Expira o payment (domínio)
		// --------------------------------------------------
		if err := domainPayment.Expire(p, now); err != nil {
			continue
		}

		if err := uc.paymentRepo.Update(ctx, p); err != nil {
			return err
		}

		uc.audit.Dispatch(audit.Event{
			BarbershopID: p.BarbershopID,
			Action:       "payment_expired",
			Entity:       "payment",
			EntityID:     &p.ID,
		})

		// --------------------------------------------------
		// 2.2️⃣ Buscar appointment associado
		// --------------------------------------------------
		ap, err := uc.appointmentRepo.GetAppointmentByID(
			ctx,
			p.AppointmentID,
		)
		if err != nil || ap == nil {
			continue
		}

		// --------------------------------------------------
		// 2.3️⃣ Cancelar appointment se estava aguardando pagamento
		// --------------------------------------------------
		if ap.Status != string(domainAppointment.StatusAwaitingPayment) {
			continue
		}

		if err := domainAppointment.Cancel(ap, now); err != nil {
			continue
		}

		if err := uc.appointmentRepo.UpdateAppointment(ctx, ap); err != nil {
			return err
		}

		// --------------------------------------------------
		// 2.4️⃣ Auditoria do cancelamento
		// --------------------------------------------------
		uc.audit.Dispatch(audit.Event{
			BarbershopID: p.BarbershopID,
			Action:       "appointment_cancelled_by_payment_expiration",
			Entity:       "appointment",
			EntityID:     &ap.ID,
		})
	}

	return nil
}
