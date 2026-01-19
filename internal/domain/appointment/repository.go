package appointment

import (
	"context"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type Repository interface {
	// -------- Barbershop --------
	GetBarbershopByID(
		ctx context.Context,
		id uint,
	) (*models.Barbershop, error)

	// -------- Product --------
	GetProduct(
		ctx context.Context,
		barbershopID uint,
		productID uint,
	) (*models.BarberProduct, error)

	// -------- Client --------
	GetOrCreateClient(
		ctx context.Context,
		barbershopID uint,
		name string,
		phone string,
		email string,
	) (*models.Client, error)

	// -------- Appointment (create / conflict) --------
	CreateAppointment(
		ctx context.Context,
		ap *models.Appointment,
	) error

	AssertNoTimeConflict(
		ctx context.Context,
		barberID uint,
		start time.Time,
		end time.Time,
	) error

	// -------- Appointment (state change) --------
	GetAppointmentForBarber(
		ctx context.Context,
		appointmentID uint,
		barberID uint,
	) (*models.Appointment, error)

	UpdateAppointment(
		ctx context.Context,
		ap *models.Appointment,
	) error

	// -------- Availability --------
	GetWorkingHours(
		ctx context.Context,
		barberID uint,
		weekday int,
	) (*models.WorkingHours, error)

	ListAppointmentsForDay(
		ctx context.Context,
		barberID uint,
		start time.Time,
		end time.Time,
	) ([]models.Appointment, error)

	IsWithinWorkingHours(
		ctx context.Context,
		barberID uint,
		start time.Time,
		end time.Time,
	) (bool, error)

	ListAppointmentsForPeriod(
		ctx context.Context,
		barberID uint,
		start time.Time,
		end time.Time,
	) ([]models.Appointment, error)
}
