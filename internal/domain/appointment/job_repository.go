package appointment

import (
	"context"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

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

	// --------------------------------------------------
	// (Opcional) se você ainda tiver algum job de lembrete
	// --------------------------------------------------
	ListAppointmentsForReminder(
		ctx context.Context,
		barbershopID uint,
		target time.Time,
	) ([]*models.Appointment, error)
}
