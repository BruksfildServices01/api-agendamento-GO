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

func (r *PaymentGormRepository) Create(
	ctx context.Context,
	p *models.Payment,
) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *PaymentGormRepository) Update(
	ctx context.Context,
	p *models.Payment,
) error {
	return r.db.WithContext(ctx).Save(p).Error
}

func (r *PaymentGormRepository) GetByID(
	ctx context.Context,
	barbershopID uint,
	id uint,
) (*models.Payment, error) {

	var p models.Payment

	err := r.db.WithContext(ctx).
		Where("id = ? AND barbershop_id = ?", id, barbershopID).
		First(&p).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (r *PaymentGormRepository) GetByAppointmentID(
	ctx context.Context,
	barbershopID uint,
	appointmentID uint,
) (*models.Payment, error) {

	var p models.Payment

	err := r.db.WithContext(ctx).
		Where("barbershop_id = ? AND appointment_id = ?", barbershopID, appointmentID).
		First(&p).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (r *PaymentGormRepository) GetByOrderID(
	ctx context.Context,
	barbershopID uint,
	orderID uint,
) (*models.Payment, error) {

	var p models.Payment

	err := r.db.WithContext(ctx).
		Where("barbershop_id = ? AND order_id = ?", barbershopID, orderID).
		Order("created_at DESC").
		First(&p).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (r *PaymentGormRepository) GetByTxID(
	ctx context.Context,
	barbershopID uint,
	txid string,
) (*models.Payment, error) {

	var p models.Payment

	err := r.db.WithContext(ctx).
		Where("barbershop_id = ? AND txid = ?", barbershopID, txid).
		First(&p).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &p, nil
}

// GLOBAL lookup (webhook only)
func (r *PaymentGormRepository) GetByTxIDGlobal(
	ctx context.Context,
	txid string,
) (*models.Payment, error) {

	var p models.Payment

	err := r.db.WithContext(ctx).
		Where("txid = ?", txid).
		First(&p).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &p, nil
}

// GLOBAL lookup by ID (MP webhook only)
func (r *PaymentGormRepository) GetByIDGlobal(
	ctx context.Context,
	id uint,
) (*models.Payment, error) {

	var p models.Payment

	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&p).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (r *PaymentGormRepository) ListExpiredPending(
	ctx context.Context,
	barbershopID uint,
	now time.Time,
) ([]*models.Payment, error) {

	var payments []*models.Payment

	err := r.db.WithContext(ctx).
		Where(
			"barbershop_id = ? AND status = ? AND expires_at < ?",
			barbershopID,
			models.PaymentStatus("pending"),
			now,
		).
		Find(&payments).
		Error

	return payments, err
}

func (r *PaymentGormRepository) ListForBarbershop(
	ctx context.Context,
	barbershopID uint,
	filter domainPayment.PaymentListFilter,
) ([]models.Payment, error) {

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
	} else {
		// Only show paid payments by default — expired and pending PIX are irrelevant to the barbershop owner.
		q = q.Where("status = ?", "paid")
	}
	if filter.StartDate != nil {
		q = q.Where("created_at >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		q = q.Where("created_at < ?", *filter.EndDate)
	}

	var payments []models.Payment

	err := q.
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&payments).
		Error

	return payments, err
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
		Total  int64 // centavos
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
		q = q.Where("created_at < ?", *to)
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
// TX REPOSITORY
// ======================================================
//

type PaymentGormTxRepository struct {
	tx *gorm.DB
}

func (r *PaymentGormTxRepository) GetByTxIDForUpdate(
	ctx context.Context,
	barbershopID uint,
	txid string,
) (*models.Payment, error) {

	var p models.Payment

	err := r.tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("barbershop_id = ? AND txid = ?", barbershopID, txid).
		First(&p).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (r *PaymentGormTxRepository) GetAppointmentForUpdate(
	ctx context.Context,
	barbershopID uint,
	appointmentID uint,
) (*models.Appointment, error) {

	var ap models.Appointment

	err := r.tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND barbershop_id = ?", appointmentID, barbershopID).
		First(&ap).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &ap, nil
}

func (r *PaymentGormTxRepository) GetOrderForUpdate(
	ctx context.Context,
	barbershopID uint,
	orderID uint,
) (*models.Order, error) {

	var order models.Order

	err := r.tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND barbershop_id = ?", orderID, barbershopID).
		First(&order).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (r *PaymentGormTxRepository) ListExpiredPendingForUpdate(
	ctx context.Context,
	barbershopID uint,
	now time.Time,
) ([]*models.Payment, error) {

	var payments []*models.Payment

	err := r.tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(
			"barbershop_id = ? AND status = ? AND expires_at < ?",
			barbershopID,
			models.PaymentStatus("pending"),
			now,
		).
		Find(&payments).
		Error

	return payments, err
}

func (r *PaymentGormTxRepository) Create(
	ctx context.Context,
	p *models.Payment,
) error {
	return r.tx.WithContext(ctx).Create(p).Error
}

func (r *PaymentGormTxRepository) MarkAsPaid(
	ctx context.Context,
	barbershopID uint,
	p *models.Payment,
) error {

	return r.tx.WithContext(ctx).
		Model(&models.Payment{}).
		Where(
			"id = ? AND barbershop_id = ? AND status = ?",
			p.ID,
			barbershopID,
			models.PaymentStatus("pending"),
		).
		Updates(map[string]any{
			"status":   p.Status,
			"paid_at":  p.PaidAt,
			"qr_code":  nil,
		}).
		Error
}

func (r *PaymentGormTxRepository) MarkAsExpired(
	ctx context.Context,
	barbershopID uint,
	p *models.Payment,
) error {

	return r.tx.WithContext(ctx).
		Model(&models.Payment{}).
		Where(
			"id = ? AND barbershop_id = ? AND status = ?",
			p.ID,
			barbershopID,
			models.PaymentStatus("pending"),
		).
		Updates(map[string]any{
			"status":  models.PaymentStatus("expired"),
			"qr_code": nil,
		}).
		Error
}

func (r *PaymentGormTxRepository) UpdateAppointmentTx(
	ctx context.Context,
	ap *models.Appointment,
) error {
	return r.tx.WithContext(ctx).Save(ap).Error
}

func (r *PaymentGormTxRepository) UpdateOrderTx(
	ctx context.Context,
	order *models.Order,
) error {
	return r.tx.WithContext(ctx).Save(order).Error
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

	return r.tx.WithContext(ctx).Create(&ev).Error
}

func (r *PaymentGormTxRepository) HasProcessedEvent(
	ctx context.Context,
	txid string,
	eventType string,
) (bool, error) {

	var count int64

	err := r.tx.WithContext(ctx).
		Model(&models.PixEvent{}).
		Where("tx_id = ? AND event_type = ?", txid, eventType).
		Count(&count).
		Error

	return count > 0, err
}

// ✅ TX lookup por order (exigido pelo TxRepository)
func (r *PaymentGormTxRepository) GetByOrderID(
	ctx context.Context,
	barbershopID uint,
	orderID uint,
) (*models.Payment, error) {

	var p models.Payment

	err := r.tx.WithContext(ctx).
		Where("barbershop_id = ? AND order_id = ?", barbershopID, orderID).
		Order("created_at DESC").
		First(&p).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &p, nil
}

// ── Subscription activation inside payment Tx ─────────────────────────────

func (r *PaymentGormTxRepository) GetSubscriptionForUpdate(
	ctx context.Context,
	id uint,
) (*models.Subscription, error) {
	var sub models.Subscription
	err := r.tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).
		First(&sub).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *PaymentGormTxRepository) GetPlanByID(
	ctx context.Context,
	id uint,
) (*models.Plan, error) {
	var plan models.Plan
	err := r.tx.WithContext(ctx).Where("id = ?", id).First(&plan).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *PaymentGormTxRepository) ActivateSubscriptionTx(
	ctx context.Context,
	id uint,
	periodStart, periodEnd time.Time,
) error {
	return r.tx.WithContext(ctx).
		Model(&models.Subscription{}).
		Where("id = ? AND status = ?", id, "pending_payment").
		Updates(map[string]any{
			"status":                  "active",
			"current_period_start":    periodStart,
			"current_period_end":      periodEnd,
			"cuts_used_in_period":     0,
			"cuts_reserved_in_period": 0,
		}).Error
}

func (r *PaymentGormTxRepository) Commit() error {
	return r.tx.Commit().Error
}

func (r *PaymentGormTxRepository) Rollback() error {
	return r.tx.Rollback().Error
}

func (r *PaymentGormTxRepository) GetByAppointmentIDForUpdate(
	ctx context.Context,
	barbershopID uint,
	appointmentID uint,
) (*models.Payment, error) {

	var p models.Payment

	err := r.tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("barbershop_id = ? AND appointment_id = ?", barbershopID, appointmentID).
		First(&p).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (r *PaymentGormTxRepository) UpdatePaymentTx(
	ctx context.Context,
	barbershopID uint,
	p *models.Payment,
) error {

	now := time.Now().UTC()

	return r.tx.WithContext(ctx).
		Model(&models.Payment{}).
		Where("id = ? AND barbershop_id = ?", p.ID, barbershopID).
		Updates(map[string]any{
			"txid":             p.TxID,
			"qr_code":          p.QRCode,
			"expires_at":       p.ExpiresAt,
			"amount":           p.Amount,
			"bundled_order_id": p.BundledOrderID,
			"updated_at":       now,
		}).
		Error
}

func (r *PaymentGormTxRepository) ListOrderItems(
	ctx context.Context,
	barbershopID uint,
	orderID uint,
) ([]models.OrderItem, error) {

	// tenant-safety: garante que o order é do tenant antes de listar items
	var count int64
	if err := r.tx.WithContext(ctx).
		Model(&models.Order{}).
		Where("id = ? AND barbershop_id = ?", orderID, barbershopID).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}

	var items []models.OrderItem
	err := r.tx.WithContext(ctx).
		Where("order_id = ?", orderID).
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *PaymentGormTxRepository) DecreaseProductStock(
	ctx context.Context,
	barbershopID uint,
	productID uint,
	quantity int,
) error {
	if quantity <= 0 || productID == 0 {
		return nil
	}

	result := r.tx.WithContext(ctx).
		Model(&models.Product{}).
		Where("id = ? AND barbershop_id = ? AND stock >= ?", productID, barbershopID, quantity).
		Update("stock", gorm.Expr("stock - ?", quantity))

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("insufficient_stock")
	}

	return nil
}

func (r *PaymentGormRepository) BeginTx(
	ctx context.Context,
	barbershopID uint,
) (domainPayment.TxRepository, error) {

	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &PaymentGormTxRepository{tx: tx}, nil
}
