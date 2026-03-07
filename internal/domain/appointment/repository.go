package appointment

import (
	"context"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type Repository interface {

	// ==================================================
	// BARBERSHOP
	// ==================================================

	GetBarbershopByID(
		ctx context.Context,
		barbershopID uint,
	) (*models.Barbershop, error)

	// ==================================================
	// PRODUCT
	// ==================================================

	GetProduct(
		ctx context.Context,
		barbershopID uint,
		productID uint,
	) (*models.BarbershopService, error)

	// ==================================================
	// CLIENT
	// ==================================================

	GetOrCreateClient(
		ctx context.Context,
		barbershopID uint,
		name string,
		phone string,
		email string,
	) (*models.Client, error)

	// ==================================================
	// APPOINTMENT - CREATE
	// ==================================================

	CreateAppointment(
		ctx context.Context,
		ap *models.Appointment,
	) error

	// ==================================================
	// TIME CONFLICT
	// ==================================================

	AssertNoTimeConflict(
		ctx context.Context,
		barbershopID uint,
		barberID uint,
		start time.Time,
		end time.Time,
	) error

	// ==================================================
	// APPOINTMENT - STATE CHANGE (OWNER FLOW)
	// ==================================================

	GetAppointmentForBarber(
		ctx context.Context,
		barbershopID uint,
		appointmentID uint,
		barberID uint,
	) (*models.Appointment, error)

	UpdateAppointment(
		ctx context.Context,
		ap *models.Appointment,
	) error

	SaveAppointmentClosure(
		ctx context.Context,
		closure *models.AppointmentClosure,
	) error

	GetAppointmentClosure(
		ctx context.Context,
		barbershopID uint,
		appointmentID uint,
	) (*models.AppointmentClosure, error)
	// ==================================================
	// AVAILABILITY
	// ==================================================

	GetWorkingHours(
		ctx context.Context,
		barbershopID uint,
		barberID uint,
		weekday int,
	) (*models.WorkingHours, error)

	ListAppointmentsForDay(
		ctx context.Context,
		barbershopID uint,
		barberID uint,
		start time.Time,
		end time.Time,
	) ([]models.Appointment, error)

	IsWithinWorkingHours(
		ctx context.Context,
		barbershopID uint,
		barberID uint,
		start time.Time,
		end time.Time,
	) (bool, error)

	ListAppointmentsForPeriod(
		ctx context.Context,
		barbershopID uint,
		barberID uint,
		start time.Time,
		end time.Time,
	) ([]models.Appointment, error)

	// ==================================================
	// REMINDER / JOB (legado - não usar para no-show)
	// ==================================================

	ListAppointmentsForReminder(
		ctx context.Context,
		barbershopID uint,
		target time.Time,
	) ([]*models.Appointment, error)

	GetAppointmentByID(
		ctx context.Context,
		barbershopID uint,
		appointmentID uint,
	) (*models.Appointment, error)

	GetOperationalSummary(
		ctx context.Context,
		barbershopID uint,
	) (*OperationalSummary, error)
}
