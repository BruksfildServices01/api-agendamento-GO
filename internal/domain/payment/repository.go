package payment

import (
	"context"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type Repository interface {

	// ------------------------------
	// 🔒 Transação (Sprint G)
	// ------------------------------
	BeginTx(ctx context.Context) (TxRepository, error)

	// ------------------------------
	// CRUD básico
	// ------------------------------
	Create(ctx context.Context, p *models.Payment) error
	Update(ctx context.Context, p *models.Payment) error

	GetByID(ctx context.Context, id uint) (*models.Payment, error)
	GetByAppointmentID(ctx context.Context, appointmentID uint) (*models.Payment, error)
	GetByTxID(ctx context.Context, txid string) (*models.Payment, error)

	// ------------------------------
	// Jobs / relatórios
	// ------------------------------
	ListExpiredPending(
		ctx context.Context,
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

// ======================================================
// TxRepository — usado SOMENTE dentro de transações
// Sprint G (PIX webhook / idempotência)
// ======================================================
type TxRepository interface {

	// 🔒 Lock pessimista por txid (webhook repetido)
	GetByTxIDForUpdate(
		ctx context.Context,
		txid string,
	) (*models.Payment, error)

	// 🔒 PRIMEIRO webhook (txid ainda não existe)
	GetSinglePendingForUpdate(
		ctx context.Context,
	) (*models.Payment, error)

	// ------------------------------
	// Escritas protegidas
	// ------------------------------
	MarkAsPaid(ctx context.Context, p *models.Payment) error
	RegisterEvent(ctx context.Context, txid string, eventType string) error

	// ------------------------------
	// Idempotência
	// ------------------------------
	HasProcessedEvent(
		ctx context.Context,
		txid string,
		eventType string,
	) (bool, error)

	// ------------------------------
	// Controle transacional
	// ------------------------------
	Commit() error
	Rollback() error
}
