package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type PaymentGormRepository struct {
	db *gorm.DB
}

func NewPaymentGormRepository(db *gorm.DB) *PaymentGormRepository {
	return &PaymentGormRepository{db: db}
}

//
// ======================================================
// ROOT REPOSITORY
// ======================================================
//

func (r *PaymentGormRepository) BeginTx(
	ctx context.Context,
) (domainPayment.TxRepository, error) {

	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &PaymentGormTxRepository{tx: tx}, nil
}

// 🔑 CREATE — CORREÇÃO DEFINITIVA DO TXID
//
// Regras:
//   - PIX payment deve nascer COM txid
//   - Se já existir payment para o appointment,
//     fazemos UPDATE explícito (nunca merge em memória)
func (r *PaymentGormRepository) Create(
	ctx context.Context,
	payment *models.Payment,
) error {

	err := r.db.WithContext(ctx).Create(payment).Error
	if err == nil {
		return nil
	}

	// Unique constraint (appointment_id ou txid)
	if errors.Is(err, gorm.ErrDuplicatedKey) {

		return r.db.WithContext(ctx).
			Model(&models.Payment{}).
			Where("appointment_id = ?", payment.AppointmentID).
			Updates(map[string]any{
				"txid":       payment.TxID,
				"expires_at": payment.ExpiresAt,
				"amount":     payment.Amount,
				"status":     payment.Status,
			}).Error
	}

	return err
}

func (r *PaymentGormRepository) Update(
	ctx context.Context,
	payment *models.Payment,
) error {
	return r.db.WithContext(ctx).Save(payment).Error
}

func (r *PaymentGormRepository) GetByID(
	ctx context.Context,
	id uint,
) (*models.Payment, error) {

	var p models.Payment
	if err := r.db.WithContext(ctx).First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *PaymentGormRepository) GetByAppointmentID(
	ctx context.Context,
	appointmentID uint,
) (*models.Payment, error) {

	var p models.Payment
	err := r.db.WithContext(ctx).
		Where("appointment_id = ?", appointmentID).
		First(&p).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &p, nil
}

func (r *PaymentGormRepository) GetByTxID(
	ctx context.Context,
	txid string,
) (*models.Payment, error) {

	var p models.Payment
	err := r.db.WithContext(ctx).
		Where("txid = ?", txid).
		First(&p).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &p, nil
}

func (r *PaymentGormRepository) ListExpiredPending(
	ctx context.Context,
	now time.Time,
) ([]*models.Payment, error) {

	var payments []*models.Payment

	err := r.db.WithContext(ctx).
		Where("status = ? AND expires_at < ?", "pending", now).
		Find(&payments).Error

	if err != nil {
		return nil, err
	}

	return payments, nil
}

func (r *PaymentGormRepository) ListForBarbershop(
	ctx context.Context,
	barbershopID uint,
	filter domainPayment.PaymentListFilter,
) ([]models.Payment, error) {

	if barbershopID == 0 {
		return []models.Payment{}, nil
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	q := r.db.WithContext(ctx).
		Model(&models.Payment{}).
		Where("barbershop_id = ?", barbershopID)

	if filter.Status != nil {
		q = q.Where("status = ?", *filter.Status)
	}
	if filter.StartDate != nil {
		q = q.Where("created_at >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		q = q.Where("created_at <= ?", *filter.EndDate)
	}

	var payments []models.Payment
	if err := q.
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&payments).
		Error; err != nil {

		return nil, err
	}

	return payments, nil
}

func (r *PaymentGormRepository) GetSummaryForBarbershop(
	ctx context.Context,
	barbershopID uint,
	from *time.Time,
	to *time.Time,
) (*domainPayment.PaymentSummary, error) {

	type row struct {
		Status string
		Count  int64
		Total  float64
	}

	var rows []row

	q := r.db.WithContext(ctx).
		Model(&models.Payment{}).
		Select("status, COUNT(*) as count, COALESCE(SUM(amount),0) as total").
		Where("barbershop_id = ?", barbershopID).
		Group("status")

	if from != nil {
		q = q.Where("created_at >= ?", *from)
	}
	if to != nil {
		q = q.Where("created_at <= ?", *to)
	}

	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}

	summary := &domainPayment.PaymentSummary{}

	for _, r := range rows {
		switch r.Status {
		case "paid":
			summary.TotalPaid = r.Total
			summary.CountPaid = r.Count
		case "pending":
			summary.TotalPending = r.Total
			summary.CountPending = r.Count
		case "expired":
			summary.TotalExpired = r.Total
			summary.CountExpired = r.Count
		}
	}

	return summary, nil
}

//
// ======================================================
// TX REPOSITORY (WEBHOOK / IDOTEMPOTÊNCIA)
// ======================================================
//

type PaymentGormTxRepository struct {
	tx *gorm.DB
}

func (r *PaymentGormTxRepository) GetSinglePendingForUpdate(
	ctx context.Context,
) (*models.Payment, error) {

	var p models.Payment

	err := r.tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("status = ?", "pending").
		Order("created_at ASC").
		Limit(1).
		First(&p).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (r *PaymentGormTxRepository) GetByTxIDForUpdate(
	ctx context.Context,
	txid string,
) (*models.Payment, error) {

	var p models.Payment
	err := r.tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("txid = ?", txid).
		First(&p).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (r *PaymentGormTxRepository) MarkAsPaid(
	ctx context.Context,
	p *models.Payment,
) error {

	return r.tx.
		Model(p).
		Where("status = ?", "pending").
		Updates(map[string]any{
			"status":  p.Status,
			"paid_at": p.PaidAt,
		}).Error
}

func (r *PaymentGormTxRepository) RegisterEvent(
	ctx context.Context,
	txid string,
	eventType string,
) error {

	ev := models.PixEvent{
		TxID:      txid,
		EventType: eventType,
	}

	return r.tx.Create(&ev).Error
}

func (r *PaymentGormTxRepository) HasProcessedEvent(
	ctx context.Context,
	txid string,
	eventType string,
) (bool, error) {

	var count int64
	err := r.tx.
		Model(&models.PixEvent{}).
		Where("tx_id = ? AND event_type = ?", txid, eventType).
		Count(&count).Error

	return count > 0, err
}

func (r *PaymentGormTxRepository) Commit() error {
	return r.tx.Commit().Error
}

func (r *PaymentGormTxRepository) Rollback() error {
	return r.tx.Rollback().Error
}
