package appointment

import (
	"context"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// AutoCompleteCandidate contém os dados mínimos necessários para que o job
// de auto-conclusão chame o use case CompleteAppointment com os defaults corretos.
type AutoCompleteCandidate struct {
	AppointmentID uint
	BarberID      uint
	// PaymentMethod derivado do pagamento existente: "pix" | "card".
	// Padrão "pix" quando não há pagamento registrado.
	PaymentMethod string
}

type JobRepository interface {
	// --------------------------------------------------
	// P0.2 — No-show candidates
	//   - candidates: scheduled + start_time <= cutoffUTC
	//   - update atomic: update only if still scheduled
	// --------------------------------------------------

	ListNoShowCandidates(
		ctx context.Context,
		barbershopID uint,
		cutoffUTC time.Time,
	) ([]*models.Appointment, error)

	// MarkNoShowAuto must be race-safe:
	// UPDATE ... WHERE id=? AND barbershop_id=? AND status='scheduled'
	// Returns (true) if it actually updated.
	MarkNoShowAuto(
		ctx context.Context,
		barbershopID uint,
		appointmentID uint,
		noShowAtUTC time.Time,
	) (bool, error)

	// ListAutoCompleteCandidates retorna agendamentos aptos para conclusão automática:
	// status IN ('scheduled', 'awaiting_payment') e end_time <= cutoffUTC.
	// Inclui o método de pagamento derivado do registro de pagamento existente
	// (pix se TxID presente, card se pago sem TxID, pix como padrão).
	ListAutoCompleteCandidates(
		ctx context.Context,
		barbershopID uint,
		cutoffUTC time.Time,
	) ([]*AutoCompleteCandidate, error)

	// --------------------------------------------------
	// (Opcional) se você ainda tiver algum job de lembrete
	// --------------------------------------------------
	ListAppointmentsForReminder(
		ctx context.Context,
		barbershopID uint,
		target time.Time,
	) ([]*models.Appointment, error)

	// CancelOrphanAwaitingPayments cancela appointments que ficaram
	// presos em awaiting_payment sem nenhum registro de payment associado.
	// Isso ocorre quando o checkout cria o appointment mas o cliente abandona
	// antes de iniciar o pagamento.
	// Retorna o número de registros cancelados.
	CancelOrphanAwaitingPayments(
		ctx context.Context,
		barbershopID uint,
		olderThan time.Time,
	) (int64, error)
}
