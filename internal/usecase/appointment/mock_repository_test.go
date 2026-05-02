package appointment

import (
	"context"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// mockRepo implementa domain.Repository para testes.
// Campos são preenchidos diretamente nos testes; métodos não configurados retornam nil/zero.
type mockRepo struct {
	shop             *models.Barbershop
	shopErr          error
	product          *models.BarbershopService
	productErr       error
	client           *models.Client
	clientErr        error
	workingHours     *models.WorkingHours
	workingHoursErr  error
	override         *models.ScheduleOverride
	overrideErr      error
	appointments     []models.Appointment
	appointmentsErr  error
	conflictErr      error
	createErr        error
	cancelExpiredErr error
}

func (r *mockRepo) GetBarbershopByID(_ context.Context, _ uint) (*models.Barbershop, error) {
	return r.shop, r.shopErr
}

func (r *mockRepo) GetProduct(_ context.Context, _, _ uint) (*models.BarbershopService, error) {
	return r.product, r.productErr
}

func (r *mockRepo) GetOrCreateClient(_ context.Context, _ uint, _, _, _ string) (*models.Client, error) {
	return r.client, r.clientErr
}

func (r *mockRepo) CreateAppointment(_ context.Context, _ *models.Appointment) error {
	return r.createErr
}

func (r *mockRepo) CreateAppointmentWithKey(_ context.Context, ap *models.Appointment, _ string) error {
	if r.createErr != nil {
		return r.createErr
	}
	ap.ID = 1
	return nil
}

func (r *mockRepo) AssertNoTimeConflict(_ context.Context, _, _ uint, _, _ time.Time) error {
	return r.conflictErr
}

func (r *mockRepo) CancelExpiredAwaitingPaymentAtSlot(_ context.Context, _, _ uint, _ time.Time) error {
	return r.cancelExpiredErr
}

func (r *mockRepo) GetAppointmentForBarber(_ context.Context, _, _, _ uint) (*models.Appointment, error) {
	return nil, nil
}

func (r *mockRepo) UpdateAppointment(_ context.Context, _ *models.Appointment) error {
	return nil
}

func (r *mockRepo) SaveAppointmentClosure(_ context.Context, _ *models.AppointmentClosure) error {
	return nil
}

func (r *mockRepo) GetAppointmentClosure(_ context.Context, _, _ uint) (*models.AppointmentClosure, error) {
	return nil, nil
}

func (r *mockRepo) GetWorkingHours(_ context.Context, _, _ uint, _ int) (*models.WorkingHours, error) {
	return r.workingHours, r.workingHoursErr
}

func (r *mockRepo) GetScheduleOverride(_ context.Context, _, _ uint, _ string, _, _, _ int) (*models.ScheduleOverride, error) {
	return r.override, r.overrideErr
}

func (r *mockRepo) ListAppointmentsForDay(_ context.Context, _, _ uint, _, _ time.Time) ([]models.Appointment, error) {
	return r.appointments, r.appointmentsErr
}

func (r *mockRepo) IsWithinWorkingHours(_ context.Context, _, _ uint, _, _ time.Time) (bool, error) {
	return true, nil
}

func (r *mockRepo) ListAppointmentsForPeriod(_ context.Context, _, _ uint, _, _ time.Time) ([]models.Appointment, error) {
	return r.appointments, r.appointmentsErr
}

func (r *mockRepo) ListAppointmentsForReminder(_ context.Context, _ uint, _ time.Time) ([]*models.Appointment, error) {
	return nil, nil
}

func (r *mockRepo) GetAppointmentByID(_ context.Context, _, _ uint) (*models.Appointment, error) {
	return nil, nil
}

func (r *mockRepo) GetOperationalSummary(_ context.Context, _ uint) (*domain.OperationalSummary, error) {
	return nil, nil
}
