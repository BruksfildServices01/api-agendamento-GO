package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
)

type AppointmentGormRepository struct {
	db *gorm.DB
}

func NewAppointmentGormRepository(db *gorm.DB) *AppointmentGormRepository {
	return &AppointmentGormRepository{db: db}
}

func (r *AppointmentGormRepository) WithTx(tx *gorm.DB) domain.Repository {
	return &AppointmentGormRepository{db: tx}
}

//
// ======================================================
// BARBERSHOP
// ======================================================
//

func (r *AppointmentGormRepository) GetBarbershopByID(
	ctx context.Context,
	barbershopID uint,
) (*models.Barbershop, error) {

	var shop models.Barbershop

	err := r.db.WithContext(ctx).
		First(&shop, barbershopID).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &shop, err
}

//
// ======================================================
// PRODUCT
// ======================================================
//

func (r *AppointmentGormRepository) GetProduct(
	ctx context.Context,
	barbershopID uint,
	productID uint,
) (*models.BarbershopService, error) {

	var product models.BarbershopService

	err := r.db.WithContext(ctx).
		Where("id = ? AND barbershop_id = ?", productID, barbershopID).
		First(&product).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &product, err
}

//
// ======================================================
// CLIENT
// ======================================================
//

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
		First(&client).
		Error

	if err == nil {
		if email != "" && client.Email != email {
			client.Email = email
			if err := r.db.WithContext(ctx).Save(&client).Error; err != nil {
				return nil, err
			}
		}
		return &client, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	client = models.Client{
		BarbershopID: &barbershopID,
		Name:         name,
		Phone:        phone,
		Email:        email,
	}

	if err := r.db.WithContext(ctx).Create(&client).Error; err != nil {
		return nil, err
	}

	return &client, nil
}

//
// ======================================================
// CREATE APPOINTMENT
// ======================================================
//

func (r *AppointmentGormRepository) CreateAppointment(
	ctx context.Context,
	ap *models.Appointment,
) error {

	err := r.db.WithContext(ctx).Create(ap).Error
	if err == nil {
		return nil
	}

	if isUniqueBarberSlotActiveViolation(err) {
		return httperr.ErrBusiness("time_conflict")
	}

	return err
}

func isUniqueBarberSlotActiveViolation(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" && pgErr.ConstraintName == "unique_barber_slot_active" {
			return true
		}
	}

	msg := err.Error()
	return strings.Contains(msg, "unique_barber_slot_active")
}

//
// ======================================================
// STATE CHANGE
// ======================================================
//

func (r *AppointmentGormRepository) GetAppointmentForBarber(
	ctx context.Context,
	barbershopID uint,
	appointmentID uint,
	barberID uint,
) (*models.Appointment, error) {

	var ap models.Appointment

	err := r.db.WithContext(ctx).
		Preload("BarberProduct").
		Preload("Client").
		Preload("Barbershop").
		Where(
			"id = ? AND barber_id = ? AND barbershop_id = ?",
			appointmentID,
			barberID,
			barbershopID,
		).
		First(&ap).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &ap, err
}

func (r *AppointmentGormRepository) UpdateAppointment(
	ctx context.Context,
	ap *models.Appointment,
) error {
	return r.db.WithContext(ctx).Save(ap).Error
}

func (r *AppointmentGormRepository) SaveAppointmentClosure(
	ctx context.Context,
	closure *models.AppointmentClosure,
) error {
	return r.db.WithContext(ctx).Create(closure).Error
}

func (r *AppointmentGormRepository) GetAppointmentClosure(
	ctx context.Context,
	barbershopID uint,
	appointmentID uint,
) (*models.AppointmentClosure, error) {

	var closure models.AppointmentClosure

	err := r.db.WithContext(ctx).
		Where("barbershop_id = ? AND appointment_id = ?", barbershopID, appointmentID).
		First(&closure).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &closure, nil
}

//
// ======================================================
// TIME CONFLICT
// ======================================================
//

func (r *AppointmentGormRepository) AssertNoTimeConflict(
	ctx context.Context,
	barbershopID uint,
	barberID uint,
	start time.Time,
	end time.Time,
) error {

	var count int64

	statuses := []models.AppointmentStatus{
		models.AppointmentStatus("scheduled"),
		models.AppointmentStatus("awaiting_payment"),
	}

	err := r.db.WithContext(ctx).
		Model(&models.Appointment{}).
		Where(
			"barbershop_id = ? AND barber_id = ? AND status IN ? AND start_time < ? AND end_time > ?",
			barbershopID,
			barberID,
			statuses,
			end,
			start,
		).
		Count(&count).
		Error

	if err != nil {
		return err
	}

	if count > 0 {
		return httperr.ErrBusiness("time_conflict")
	}

	return nil
}

//
// ======================================================
// WORKING HOURS
// ======================================================
//

func (r *AppointmentGormRepository) GetWorkingHours(
	ctx context.Context,
	barbershopID uint,
	barberID uint,
	weekday int,
) (*models.WorkingHours, error) {

	var wh models.WorkingHours

	err := r.db.WithContext(ctx).
		Where(
			"barbershop_id = ? AND barber_id = ? AND weekday = ?",
			barbershopID,
			barberID,
			weekday,
		).
		First(&wh).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &wh, err
}

// DEPRECATED: timezone-unsafe.
func (r *AppointmentGormRepository) IsWithinWorkingHours(
	ctx context.Context,
	barbershopID uint,
	barberID uint,
	start time.Time,
	end time.Time,
) (bool, error) {

	weekday := int(start.Weekday())

	wh, err := r.GetWorkingHours(ctx, barbershopID, barberID, weekday)
	if err != nil || wh == nil {
		return false, err
	}

	if !wh.Active || wh.StartTime == "" || wh.EndTime == "" {
		return false, nil
	}

	loc := start.Location()

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

//
// ======================================================
// LIST
// ======================================================
//

func (r *AppointmentGormRepository) ListAppointmentsForDay(
	ctx context.Context,
	barbershopID uint,
	barberID uint,
	start time.Time,
	end time.Time,
) ([]models.Appointment, error) {

	var apps []models.Appointment

	err := r.db.WithContext(ctx).
		Where(
			"barbershop_id = ? AND barber_id = ? AND start_time >= ? AND start_time < ? AND status NOT IN ?",
			barbershopID,
			barberID,
			start,
			end,
			[]string{"cancelled", "no_show"},
		).
		Order("start_time ASC").
		Find(&apps).
		Error

	return apps, err
}

func (r *AppointmentGormRepository) ListAppointmentsForPeriod(
	ctx context.Context,
	barbershopID uint,
	barberID uint,
	start time.Time,
	end time.Time,
) ([]models.Appointment, error) {

	var apps []models.Appointment

	err := r.db.WithContext(ctx).
		Preload("Client").
		Preload("BarberProduct").
		Where(
			"barbershop_id = ? AND barber_id = ? AND start_time >= ? AND start_time < ?",
			barbershopID,
			barberID,
			start,
			end,
		).
		Order("start_time ASC").
		Find(&apps).
		Error

	return apps, err
}

//
// ======================================================
// REMINDER
// ======================================================
//

func (r *AppointmentGormRepository) ListAppointmentsForReminder(
	ctx context.Context,
	barbershopID uint,
	target time.Time,
) ([]*models.Appointment, error) {

	start := target.Add(-5 * time.Minute)
	end := target.Add(5 * time.Minute)

	var apps []*models.Appointment

	err := r.db.WithContext(ctx).
		Preload("Client").
		Preload("Barbershop").
		Where(
			"barbershop_id = ? AND start_time BETWEEN ? AND ? AND status = ?",
			barbershopID,
			start,
			end,
			models.AppointmentStatus("scheduled"),
		).
		Find(&apps).
		Error

	return apps, err
}

func (r *AppointmentGormRepository) GetAppointmentByID(
	ctx context.Context,
	barbershopID uint,
	appointmentID uint,
) (*models.Appointment, error) {

	var ap models.Appointment

	err := r.db.WithContext(ctx).
		Preload("Client").
		Preload("BarberProduct").
		Preload("Barbershop").
		Where("id = ? AND barbershop_id = ?", appointmentID, barbershopID).
		First(&ap).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &ap, err
}

//
// ======================================================
// BARBERSHOP LISTER
// ======================================================
//

func (r *AppointmentGormRepository) ListBarbershops(
	ctx context.Context,
) ([]domain.BarbershopInfo, error) {

	type result struct {
		ID       uint
		Timezone string
	}

	var rows []result

	err := r.db.WithContext(ctx).
		Model(&models.Barbershop{}).
		Select("id, timezone").
		Find(&rows).
		Error

	if err != nil {
		return nil, err
	}

	shops := make([]domain.BarbershopInfo, 0, len(rows))

	for _, row := range rows {
		shops = append(shops, domain.BarbershopInfo{
			ID:       row.ID,
			Timezone: row.Timezone,
		})
	}

	return shops, nil
}

//
// ======================================================
// OPERATIONAL SUMMARY (FINANCE SAFE)
// ======================================================
//

func (r *AppointmentGormRepository) GetOperationalSummary(
	ctx context.Context,
	barbershopID uint,
) (*domain.OperationalSummary, error) {

	type resultRow struct {
		TotalReceived     int64
		CountCompleted    int64
		CountProductsSold int64
	}

	var row resultRow

	err := r.db.WithContext(ctx).Raw(`
		SELECT
			(SELECT COALESCE(SUM(p.amount), 0)
			 FROM payments p
			 WHERE p.barbershop_id = ?
			   AND p.status = 'paid'
			) AS total_received,

			COUNT(*) FILTER (WHERE a.status = 'completed') AS count_completed,

			(SELECT COALESCE(SUM(oi.quantity), 0)
			 FROM order_items oi
			 JOIN orders o ON o.id = oi.order_id
			 WHERE o.barbershop_id = ?
			   AND o.status = 'paid'
			) AS count_products_sold

		FROM appointments a
		WHERE a.barbershop_id = ?
	`, barbershopID, barbershopID, barbershopID).Scan(&row).Error

	if err != nil {
		return nil, err
	}

	return &domain.OperationalSummary{
		TotalReceived:     row.TotalReceived,
		CountCompleted:    int(row.CountCompleted),
		CountProductsSold: int(row.CountProductsSold),
	}, nil
}

// ======================================================
// JOB REPOSITORY (P0.2 - race-safe no-show)
// ======================================================

func (r *AppointmentGormRepository) ListNoShowCandidates(
	ctx context.Context,
	barbershopID uint,
	cutoffUTC time.Time,
) ([]*models.Appointment, error) {

	var apps []*models.Appointment

	err := r.db.WithContext(ctx).
		Model(&models.Appointment{}).
		Select("id, client_id, status").
		Where(
			"barbershop_id = ? AND status = ? AND start_time <= ?",
			barbershopID,
			models.AppointmentStatusScheduled,
			cutoffUTC,
		).
		Order("start_time ASC").
		Limit(500).
		Find(&apps).Error

	return apps, err
}

func (r *AppointmentGormRepository) MarkNoShowAuto(
	ctx context.Context,
	barbershopID uint,
	appointmentID uint,
	noShowAt time.Time,
) (bool, error) {

	res := r.db.WithContext(ctx).
		Model(&models.Appointment{}).
		Where(
			"id = ? AND barbershop_id = ? AND status = ?",
			appointmentID,
			barbershopID,
			models.AppointmentStatusScheduled,
		).
		Updates(map[string]any{
			"status":         models.AppointmentStatusNoShow,
			"no_show_at":     noShowAt,
			"no_show_source": models.NoShowSourceAuto,
		})

	if res.Error != nil {
		return false, res.Error
	}

	return res.RowsAffected > 0, nil
}

var _ domain.Repository = (*AppointmentGormRepository)(nil)
var _ domain.BarbershopLister = (*AppointmentGormRepository)(nil)
var _ domain.JobRepository = (*AppointmentGormRepository)(nil)
