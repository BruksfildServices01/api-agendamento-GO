package payment

import (
	"context"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type Repository interface {
	GetByTxIDGlobal(
		ctx context.Context,
		txid string,
	) (*models.Payment, error)

	BeginTx(
		ctx context.Context,
		barbershopID uint,
	) (TxRepository, error)

	Create(
		ctx context.Context,
		p *models.Payment,
	) error

	Update(
		ctx context.Context,
		p *models.Payment,
	) error

	GetByID(
		ctx context.Context,
		barbershopID uint,
		id uint,
	) (*models.Payment, error)

	GetByAppointmentID(
		ctx context.Context,
		barbershopID uint,
		appointmentID uint,
	) (*models.Payment, error)

	GetByOrderID(
		ctx context.Context,
		barbershopID uint,
		orderID uint,
	) (*models.Payment, error)

	GetByTxID(
		ctx context.Context,
		barbershopID uint,
		txid string,
	) (*models.Payment, error)

	ListExpiredPending(
		ctx context.Context,
		barbershopID uint,
		now time.Time,
	) ([]*models.Payment, error)

	ListForBarbershop(
		ctx context.Context,
		barbershopID uint,
		filter PaymentListFilter,
	) ([]models.Payment, error)

	GetSummaryForBarbershop(
		ctx context.Context,
		barbershopID uint,
		from *time.Time,
		to *time.Time,
	) (*PaymentSummary, error)
}

type TxRepository interface {
	GetByTxIDForUpdate(
		ctx context.Context,
		barbershopID uint,
		txid string,
	) (*models.Payment, error)

	GetByAppointmentIDForUpdate(
		ctx context.Context,
		barbershopID uint,
		appointmentID uint,
	) (*models.Payment, error)

	GetAppointmentForUpdate(
		ctx context.Context,
		barbershopID uint,
		appointmentID uint,
	) (*models.Appointment, error)

	GetOrderForUpdate(
		ctx context.Context,
		barbershopID uint,
		orderID uint,
	) (*models.Order, error)

	ListExpiredPendingForUpdate(
		ctx context.Context,
		barbershopID uint,
		now time.Time,
	) ([]*models.Payment, error)

	Create(
		ctx context.Context,
		p *models.Payment,
	) error

	MarkAsPaid(
		ctx context.Context,
		barbershopID uint,
		p *models.Payment,
	) error

	MarkAsExpired(
		ctx context.Context,
		barbershopID uint,
		p *models.Payment,
	) error

	UpdatePaymentTx(
		ctx context.Context,
		barbershopID uint,
		p *models.Payment,
	) error

	UpdateAppointmentTx(
		ctx context.Context,
		ap *models.Appointment,
	) error

	UpdateOrderTx(
		ctx context.Context,
		order *models.Order,
	) error

	RegisterEvent(
		ctx context.Context,
		txid string,
		eventType string,
	) error

	HasProcessedEvent(
		ctx context.Context,
		txid string,
		eventType string,
	) (bool, error)

	GetByOrderID(
		ctx context.Context,
		barbershopID uint,
		orderID uint,
	) (*models.Payment, error)

	ListOrderItems(
		ctx context.Context,
		barbershopID uint,
		orderID uint,
	) ([]models.OrderItem, error)

	DecreaseProductStock(
		ctx context.Context,
		barbershopID uint,
		productID uint,
		quantity int,
	) error

	Commit() error
	Rollback() error
}
