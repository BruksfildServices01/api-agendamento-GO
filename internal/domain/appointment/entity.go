package appointment

import (
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// ===============================
// Domain Actions
// ===============================

func Cancel(ap *models.Appointment, now time.Time) error {
	if err := CanCancel(Status(ap.Status)); err != nil {
		return err
	}

	ap.Status = string(StatusCancelled)
	ap.CancelledAt = &now
	return nil
}

func Complete(ap *models.Appointment, now time.Time) error {
	if err := CanComplete(Status(ap.Status)); err != nil {
		return err
	}

	ap.Status = string(StatusCompleted)
	ap.CompletedAt = &now
	return nil
}
