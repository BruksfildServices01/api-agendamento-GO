package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type AppointmentGormRepository struct {
	db *gorm.DB
}

func NewAppointmentGormRepository(db *gorm.DB) *AppointmentGormRepository {
	return &AppointmentGormRepository{db: db}
}

// --------------------------------------------------
// Barbershop
// --------------------------------------------------

func (r *AppointmentGormRepository) GetBarbershopByID(
	ctx context.Context,
	id uint,
) (*models.Barbershop, error) {

	var shop models.Barbershop
	if err := r.db.WithContext(ctx).First(&shop, id).Error; err != nil {
		return nil, err
	}
	return &shop, nil
}

// --------------------------------------------------
// Product
// --------------------------------------------------

func (r *AppointmentGormRepository) GetProduct(
	ctx context.Context,
	barbershopID uint,
	productID uint,
) (*models.BarberProduct, error) {

	var product models.BarberProduct
	if err := r.db.WithContext(ctx).
		Where("id = ? AND barbershop_id = ?", productID, barbershopID).
		First(&product).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

// --------------------------------------------------
// Client
// --------------------------------------------------

func (r *AppointmentGormRepository) FindClientByPhone(
	ctx context.Context,
	barbershopID uint,
	phone string,
) (*models.Client, error) {

	var client models.Client
	if err := r.db.WithContext(ctx).
		Where("barbershop_id = ? AND phone = ?", barbershopID, phone).
		First(&client).Error; err != nil {
		return nil, err
	}
	return &client, nil
}

func (r *AppointmentGormRepository) CreateClient(
	ctx context.Context,
	client *models.Client,
) error {
	return r.db.WithContext(ctx).Create(client).Error
}

// --------------------------------------------------
// Appointment
// --------------------------------------------------

func (r *AppointmentGormRepository) HasTimeConflict(
	ctx context.Context,
	barberID uint,
	start time.Time,
	end time.Time,
) (bool, error) {

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.Appointment{}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(
			"barber_id = ? AND status = 'scheduled' AND start_time < ? AND end_time > ?",
			barberID,
			end,
			start,
		).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *AppointmentGormRepository) CreateAppointment(
	ctx context.Context,
	ap *models.Appointment,
) error {
	return r.db.WithContext(ctx).Create(ap).Error
}

// --------------------------------------------------
// Appointment (Cancel / Complete)
// --------------------------------------------------

func (r *AppointmentGormRepository) GetAppointmentForBarber(
	ctx context.Context,
	appointmentID uint,
	barberID uint,
) (*models.Appointment, error) {

	var ap models.Appointment
	if err := r.db.WithContext(ctx).
		Where("id = ? AND barber_id = ?", appointmentID, barberID).
		First(&ap).Error; err != nil {
		return nil, err
	}

	return &ap, nil
}

func (r *AppointmentGormRepository) UpdateAppointment(
	ctx context.Context,
	ap *models.Appointment,
) error {
	return r.db.WithContext(ctx).Save(ap).Error
}

// --------------------------------------------------
// Availability
// --------------------------------------------------

func (r *AppointmentGormRepository) GetWorkingHours(
	ctx context.Context,
	barberID uint,
	weekday int,
) (*models.WorkingHours, error) {

	var wh models.WorkingHours
	if err := r.db.WithContext(ctx).
		Where("barber_id = ? AND weekday = ?", barberID, weekday).
		First(&wh).Error; err != nil {
		return nil, err
	}

	return &wh, nil
}

func (r *AppointmentGormRepository) ListAppointmentsForDay(
	ctx context.Context,
	barberID uint,
	start time.Time,
	end time.Time,
) ([]models.Appointment, error) {

	var apps []models.Appointment
	if err := r.db.WithContext(ctx).
		Select("start_time", "end_time").
		Where(
			"barber_id = ? AND status = 'scheduled' AND start_time >= ? AND start_time < ?",
			barberID, start, end,
		).
		Order("start_time ASC").
		Find(&apps).Error; err != nil {
		return nil, err
	}

	return apps, nil
}

func (r *AppointmentGormRepository) IsWithinWorkingHours(
	ctx context.Context,
	barberID uint,
	start time.Time,
	end time.Time,
) (bool, error) {

	weekday := int(start.Weekday())
	loc := start.Location()

	var wh models.WorkingHours
	if err := r.db.WithContext(ctx).
		Where("barber_id = ? AND weekday = ?", barberID, weekday).
		First(&wh).Error; err != nil {
		return false, nil
	}

	if !wh.Active || wh.StartTime == "" || wh.EndTime == "" {
		return false, nil
	}

	parseHM := func(hm string) time.Time {
		t, _ := time.Parse("15:04", hm)
		return time.Date(
			start.Year(), start.Month(), start.Day(),
			t.Hour(), t.Minute(), 0, 0,
			loc,
		)
	}

	workStart := parseHM(wh.StartTime)
	workEnd := parseHM(wh.EndTime)

	if start.Before(workStart) || end.After(workEnd) {
		return false, nil
	}

	if wh.LunchStart != "" && wh.LunchEnd != "" {
		lunchStart := parseHM(wh.LunchStart)
		lunchEnd := parseHM(wh.LunchEnd)
		if start.Before(lunchEnd) && end.After(lunchStart) {
			return false, nil
		}
	}

	return true, nil
}

func (r *AppointmentGormRepository) GetOrCreateClient(
	ctx context.Context,
	barbershopID uint,
	name string,
	phone string,
	email string,
) (*models.Client, error) {

	var client models.Client
	err := r.db.WithContext(ctx).
		Where("barbershop_id = ? AND phone = ?", barbershopID, phone).
		First(&client).Error

	if err == nil {
		return &client, nil
	}

	client = models.Client{
		BarbershopID: barbershopID,
		Name:         name,
		Phone:        phone,
		Email:        email,
	}

	if err := r.db.WithContext(ctx).Create(&client).Error; err != nil {
		return nil, err
	}

	return &client, nil
}

func (r *AppointmentGormRepository) AssertNoTimeConflict(
	ctx context.Context,
	barberID uint,
	start time.Time,
	end time.Time,
) error {

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.Appointment{}).
		Where(
			"barber_id = ? AND status = 'scheduled' AND start_time < ? AND end_time > ?",
			barberID,
			end,
			start,
		).
		Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		return httperr.ErrBusiness("time_conflict")
	}

	return nil
}

func (r *AppointmentGormRepository) ListAppointmentsForPeriod(
	ctx context.Context,
	barberID uint,
	start time.Time,
	end time.Time,
) ([]models.Appointment, error) {

	var apps []models.Appointment

	err := r.db.WithContext(ctx).
		Preload("Client").
		Preload("BarberProduct").
		Where(
			"barber_id = ? AND start_time >= ? AND start_time < ?",
			barberID,
			start,
			end,
		).
		Order("start_time ASC").
		Find(&apps).Error

	if err != nil {
		return nil, err
	}

	return apps, nil
}

// Compile-time check
var _ domain.Repository = (*AppointmentGormRepository)(nil)
