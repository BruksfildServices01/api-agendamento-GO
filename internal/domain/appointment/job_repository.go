package appointment

import (
	"context"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// JobRepository é usado por jobs, PIX, webhooks e automações.
// Não depende de identidade humana.
type JobRepository interface {
	GetAppointmentByID(
		ctx context.Context,
		appointmentID uint,
	) (*models.Appointment, error)

	UpdateAppointment(
		ctx context.Context,
		ap *models.Appointment,
	) error

	ListAppointmentsForReminder(
		ctx context.Context,
		target time.Time,
	) ([]*models.Appointment, error)
}
