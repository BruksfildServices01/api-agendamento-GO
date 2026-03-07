package appointment

import (
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

func Cancel(ap *models.Appointment, now time.Time) error {
	if err := CanCancel(Status(ap.Status)); err != nil {
		return err
	}

	ap.Status = models.AppointmentStatus(StatusCancelled)
	ap.CancelledAt = &now
	return nil
}

func Complete(ap *models.Appointment, now time.Time) error {
	if err := CanComplete(Status(ap.Status)); err != nil {
		return err
	}

	ap.Status = models.AppointmentStatus(StatusCompleted)
	ap.CompletedAt = &now
	return nil
}
