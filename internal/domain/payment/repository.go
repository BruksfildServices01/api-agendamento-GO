package payment

import (
	"context"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

//
// ======================================================
// ROOT REPOSITORY (Multi-tenant obrigatório)
// ======================================================
//

type Repository interface {

	// ==================================================
	// 🔎 GLOBAL LOOKUP (EXCEÇÃO CONTROLADA)
	// ==================================================
	// Usado apenas para webhook PIX para descobrir o barbershopID.
	// Nunca deve ser usado para update.
	GetByTxIDGlobal(
		ctx context.Context,
		txid string,
	) (*models.Payment, error)

	// ==================================================
	// 🔒 Transação (sempre escopada por tenant)
	// ==================================================

	BeginTx(
		ctx context.Context,
		barbershopID uint,
	) (TxRepository, error)

	// ==================================================
	// CRUD básico (sempre escopado por tenant)
	// ==================================================

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

	// ✅ suporte a Order
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

	// ==================================================
	// Jobs / relatórios
	// ==================================================

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

//
// ======================================================
// TX REPOSITORY
// Multi-tenant obrigatório dentro da transação
// ======================================================
//

type TxRepository interface {

	// ==================================================
	// 🔒 Locks pessimistas
	// ==================================================

	GetByTxIDForUpdate(
		ctx context.Context,
		barbershopID uint,
		txid string,
	) (*models.Payment, error)

	// ✅ NOVO — lock do payment por appointment (resolve corrida do PIX)
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

	// ✅ Lock pessimista de Order
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

	// ==================================================
	// Escritas dentro da TX
	// ==================================================

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

	// ✅ NOVO — update do payment dentro da TX (txid/qr/expires)
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

	// ==================================================
	// Idempotência por evento
	// ==================================================

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

	// Order items (para restock no expire)
	ListOrderItems(
		ctx context.Context,
		barbershopID uint,
		orderID uint,
	) ([]models.OrderItem, error)

	// Restock transacional (só pra rollback/expire)
	IncreaseProductStock(
		ctx context.Context,
		barbershopID uint,
		productID uint,
		quantity int,
	) error

	// ==================================================
	// Controle transacional
	// ==================================================

	Commit() error
	Rollback() error
}
