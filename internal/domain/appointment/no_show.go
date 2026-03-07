package appointment

import (
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

func MarkNoShow(
	ap *models.Appointment,
	now time.Time,
	source string,
) error {

	if err := CanMarkNoShow(Status(ap.Status)); err != nil {
		return err
	}

	ap.Status = models.AppointmentStatus(StatusNoShow)

	ap.NoShowAt = &now

	src := models.NoShowSourceType(source)
	ap.NoShowSource = &src

	ap.CompletedAt = nil
	ap.CancelledAt = nil

	return nil
}
