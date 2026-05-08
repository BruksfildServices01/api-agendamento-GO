package payment

// Testes de integração (sem DB real) para o fluxo de ativação de assinatura
// via mark_mp_payment_as_paid.Execute.
//
// Abordagem:
//   - mockPaymentRepo  implementa domainPayment.Repository (camada de topo)
//   - mockTxRepo       implementa domainPayment.TxRepository (dentro da transação)
//   - mockIdemStore    implementa idempotency.Store
//   - noopNotifier*    implementam as interfaces de notificação
//
// Para o audit.Dispatcher, usamos gorm.Open com DryRun=true: o Create é
// compilado em SQL mas nunca executado, sem chamada de rede.
// Se o gorm.Open falhar no ambiente de teste, os casos que dependem do Dispatcher
// são pulados com t.Skip — os demais (lógica pura) continuam.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domainNotification "github.com/BruksfildServices01/barber-scheduler/internal/domain/notification"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	domainTicket "github.com/BruksfildServices01/barber-scheduler/internal/domain/ticket"
	"github.com/BruksfildServices01/barber-scheduler/internal/idempotency"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// ── noop SQL driver ───────────────────────────────────────────────────────────────
//
// Driver mínimo que satisfaz database/sql/driver sem qualquer conexão de rede.
// Registrado via init() para ser compartilhado por todos os testes do pacote.
// O GORM postgres dialector aceita um *sql.DB existente (Config.Conn), portanto
// conseguimos criar um audit.Dispatcher sem conectar a um postgres real.

const noopDriverName = "noop-payment-test"

func init() {
	sql.Register(noopDriverName, &noopSQLDriver{})
}

type noopSQLDriver struct{}
type noopSQLConn struct{}
type noopSQLStmt struct{}
type noopSQLRows struct{}
type noopSQLTx struct{}
type noopSQLResult struct{}

func (*noopSQLDriver) Open(_ string) (driver.Conn, error) { return &noopSQLConn{}, nil }
func (*noopSQLConn) Prepare(_ string) (driver.Stmt, error) { return &noopSQLStmt{}, nil }
func (*noopSQLConn) Close() error                          { return nil }
func (*noopSQLConn) Begin() (driver.Tx, error)             { return &noopSQLTx{}, nil }
func (*noopSQLTx) Commit() error                           { return nil }
func (*noopSQLTx) Rollback() error                         { return nil }
func (*noopSQLStmt) Close() error                          { return nil }
func (*noopSQLStmt) NumInput() int                         { return -1 }
func (*noopSQLStmt) Exec(_ []driver.Value) (driver.Result, error) {
	return noopSQLResult{}, nil
}
func (*noopSQLStmt) Query(_ []driver.Value) (driver.Rows, error) { return &noopSQLRows{}, nil }
func (noopSQLResult) LastInsertId() (int64, error)                { return 0, nil }
func (noopSQLResult) RowsAffected() (int64, error)                { return 1, nil }
func (*noopSQLRows) Columns() []string                            { return nil }
func (*noopSQLRows) Close() error                                 { return nil }
func (*noopSQLRows) Next(_ []driver.Value) error                  { return io.EOF }

// ── audit dispatcher para testes ─────────────────────────────────────────────────

// newTestDispatcher cria um Dispatcher cujo Logger usa o noop SQL driver:
// db.Create compila o SQL mas chama nosso noop driver que retorna EOF imediatamente.
// A goroutine worker pode logar "[audit] log error: sql: no rows in result set"
// no stderr (comportamento aceitável em testes — não causa falha).
// t.Cleanup chama Shutdown para fechar a goroutine worker de forma limpa.
func newTestDispatcher(t *testing.T) *audit.Dispatcher {
	t.Helper()

	sqlDB, err := sql.Open(noopDriverName, "")
	if err != nil {
		t.Fatalf("sql.Open noop driver: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	// postgres.New com Conn já existente nunca tenta conectar ao postgres real.
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open noop: %v", err)
	}

	d := audit.NewDispatcher(audit.New(db))
	t.Cleanup(d.Shutdown)
	return d
}

// ── mockPaymentRepo ───────────────────────────────────────────────────────────────

type mockPaymentRepo struct {
	payment *models.Payment
	txRepo  *mockTxRepo
}

func (r *mockPaymentRepo) GetByIDGlobal(_ context.Context, _ uint) (*models.Payment, error) {
	return r.payment, nil
}
func (r *mockPaymentRepo) BeginTx(_ context.Context, _ uint) (domainPayment.TxRepository, error) {
	return r.txRepo, nil
}
func (r *mockPaymentRepo) Create(_ context.Context, _ *models.Payment) error   { return nil }
func (r *mockPaymentRepo) Update(_ context.Context, _ *models.Payment) error   { return nil }
func (r *mockPaymentRepo) GetByTxIDGlobal(_ context.Context, _ string) (*models.Payment, error) {
	return r.payment, nil
}
func (r *mockPaymentRepo) GetByID(_ context.Context, _, _ uint) (*models.Payment, error) {
	return r.payment, nil
}
func (r *mockPaymentRepo) GetByAppointmentID(_ context.Context, _, _ uint) (*models.Payment, error) {
	return nil, nil
}
func (r *mockPaymentRepo) GetByOrderID(_ context.Context, _, _ uint) (*models.Payment, error) {
	return nil, nil
}
func (r *mockPaymentRepo) GetByTxID(_ context.Context, _ uint, _ string) (*models.Payment, error) {
	return nil, nil
}
func (r *mockPaymentRepo) ListExpiredPending(_ context.Context, _ uint, _ time.Time) ([]*models.Payment, error) {
	return nil, nil
}
func (r *mockPaymentRepo) ListForBarbershop(_ context.Context, _ uint, _ domainPayment.PaymentListFilter) ([]models.Payment, error) {
	return nil, nil
}
func (r *mockPaymentRepo) CountForBarbershop(_ context.Context, _ uint, _ domainPayment.PaymentListFilter) (int64, error) {
	return 0, nil
}
func (r *mockPaymentRepo) GetSummaryForBarbershop(_ context.Context, _ uint, _, _ *time.Time) (*domainPayment.PaymentSummary, error) {
	return nil, nil
}

// ── mockTxRepo ────────────────────────────────────────────────────────────────────

type mockTxRepo struct {
	mu sync.Mutex

	// Payment retornado por GetByTxIDForUpdate
	payment *models.Payment

	// Subscription e plan retornados pelos métodos de ativação
	sub  *models.Subscription
	plan *models.Plan

	// Appointment e order retornados para os caminhos de agendamento/pedido
	appointment *models.Appointment
	order       *models.Order

	// Registro de chamadas
	markedAsPaid        bool
	activatedSubID      uint
	activatedPeriodStart time.Time
	activatedPeriodEnd   time.Time
	committedCount      int
	rolledBackCount     int
	registeredEvent     bool

	// Controles configuráveis por teste
	hasProcessedEvent bool
	commitErr         error
}

func (r *mockTxRepo) GetByTxIDForUpdate(_ context.Context, _ uint, _ string) (*models.Payment, error) {
	return r.payment, nil
}
func (r *mockTxRepo) GetByAppointmentIDForUpdate(_ context.Context, _, _ uint) (*models.Payment, error) {
	return r.payment, nil
}
func (r *mockTxRepo) GetAppointmentForUpdate(_ context.Context, _, _ uint) (*models.Appointment, error) {
	return r.appointment, nil
}
func (r *mockTxRepo) GetOrderForUpdate(_ context.Context, _, _ uint) (*models.Order, error) {
	return r.order, nil
}
func (r *mockTxRepo) ListExpiredPendingForUpdate(_ context.Context, _ uint, _ time.Time) ([]*models.Payment, error) {
	return nil, nil
}
func (r *mockTxRepo) Create(_ context.Context, _ *models.Payment) error { return nil }
func (r *mockTxRepo) MarkAsPaid(_ context.Context, _ uint, _ *models.Payment) error {
	r.mu.Lock()
	r.markedAsPaid = true
	r.mu.Unlock()
	return nil
}
func (r *mockTxRepo) MarkAsExpired(_ context.Context, _ uint, _ *models.Payment) error { return nil }
func (r *mockTxRepo) UpdatePaymentTx(_ context.Context, _ uint, _ *models.Payment) error {
	return nil
}
func (r *mockTxRepo) UpdateAppointmentTx(_ context.Context, _ *models.Appointment) error {
	return nil
}
func (r *mockTxRepo) UpdateOrderTx(_ context.Context, _ *models.Order) error { return nil }
func (r *mockTxRepo) RegisterEvent(_ context.Context, _, _ string) error {
	r.mu.Lock()
	r.registeredEvent = true
	r.mu.Unlock()
	return nil
}
func (r *mockTxRepo) HasProcessedEvent(_ context.Context, _, _ string) (bool, error) {
	return r.hasProcessedEvent, nil
}
func (r *mockTxRepo) GetByOrderID(_ context.Context, _, _ uint) (*models.Payment, error) {
	return nil, nil
}
func (r *mockTxRepo) ListOrderItems(_ context.Context, _, _ uint) ([]models.OrderItem, error) {
	return nil, nil
}
func (r *mockTxRepo) DecreaseProductStock(_ context.Context, _, _ uint, _ int) error { return nil }
func (r *mockTxRepo) GetSubscriptionForUpdate(_ context.Context, _ uint) (*models.Subscription, error) {
	return r.sub, nil
}
func (r *mockTxRepo) GetPlanByID(_ context.Context, _ uint) (*models.Plan, error) {
	return r.plan, nil
}
func (r *mockTxRepo) ActivateSubscriptionTx(_ context.Context, id uint, start, end time.Time) error {
	r.mu.Lock()
	r.activatedSubID = id
	r.activatedPeriodStart = start
	r.activatedPeriodEnd = end
	r.mu.Unlock()
	return nil
}
func (r *mockTxRepo) Commit() error {
	r.mu.Lock()
	r.committedCount++
	r.mu.Unlock()
	return r.commitErr
}
func (r *mockTxRepo) Rollback() error {
	r.mu.Lock()
	r.rolledBackCount++
	r.mu.Unlock()
	return nil
}

// ── noop helpers ──────────────────────────────────────────────────────────────────

type noopIdemStore struct{ exists bool }

func (s *noopIdemStore) Exists(_ context.Context, _ string) (bool, error) { return s.exists, nil }
func (s *noopIdemStore) Save(_ context.Context, _ string) error            { return nil }

type noopNotifier struct{}

func (n *noopNotifier) Notify(_ context.Context, _ domainNotification.PaymentConfirmedInput) error {
	return nil
}

type noopApptNotifier struct{}

func (n *noopApptNotifier) NotifyConfirmed(_ context.Context, _ domainNotification.AppointmentConfirmedInput) error {
	return nil
}
func (n *noopApptNotifier) NotifyCancelled(_ context.Context, _ domainNotification.AppointmentCancelledInput) error {
	return nil
}
func (n *noopApptNotifier) NotifyRescheduled(_ context.Context, _ domainNotification.AppointmentRescheduledInput) error {
	return nil
}

type noopTicketRepo struct{}

func (r *noopTicketRepo) Upsert(_ context.Context, _ *models.AppointmentTicket) error  { return nil }
func (r *noopTicketRepo) GetByToken(_ context.Context, _ string) (*models.AppointmentTicket, error) {
	return nil, nil
}
func (r *noopTicketRepo) GetByAppointmentID(_ context.Context, _ uint) (*models.AppointmentTicket, error) {
	return nil, nil
}
func (r *noopTicketRepo) Save(_ context.Context, _ *models.AppointmentTicket) error { return nil }

// ── factory ───────────────────────────────────────────────────────────────────────

func newUC(t *testing.T, repo domainPayment.Repository, idem idempotency.Store) *MarkMPPaymentAsPaid {
	t.Helper()
	d := newTestDispatcher(t)
	return NewMarkMPPaymentAsPaid(
		repo,
		d,
		&noopNotifier{},
		idem,
		nil, // db não é usado nos caminhos testados (sem notificação de agendamento)
		&noopApptNotifier{},
		&noopTicketRepo{},
		"http://app.test",
	)
}

// ── helpers para construção de modelos ───────────────────────────────────────────

func uint64p(v uint) *uint  { return &v }
func strp(v string) *string { return &v }

func pendingPayment(id uint, subID uint, txid string) *models.Payment {
	return &models.Payment{
		ID:             id,
		BarbershopID:   1,
		Status:         "pending",
		TxID:           strp(txid),
		SubscriptionID: uint64p(subID),
	}
}

func pendingSubscription(id, planID uint) *models.Subscription {
	return &models.Subscription{
		ID:     id,
		PlanID: planID,
		Status: "pending_payment",
	}
}

func planWith(id uint, durationDays int) *models.Plan {
	return &models.Plan{ID: id, DurationDays: durationDays}
}

// ── testes ────────────────────────────────────────────────────────────────────────

// TestMarkMPPaid_Subscription_ActivatesOnConfirmation verifica o caminho principal:
// payment com SubscriptionID → MarkAsPaid + ActivateSubscriptionTx na mesma tx.
func TestMarkMPPaid_Subscription_ActivatesOnConfirmation(t *testing.T) {
	const (
		paymentID  = uint(1)
		subID      = uint(10)
		planID     = uint(5)
		txid       = "sub_pending:10:1234567890"
		providerID = "QRC_ABCDEF"
		durationDays = 30
	)

	pmt := pendingPayment(paymentID, subID, txid)
	sub := pendingSubscription(subID, planID)
	plan := planWith(planID, durationDays)

	txRepo := &mockTxRepo{payment: pmt, sub: sub, plan: plan}
	repo := &mockPaymentRepo{payment: pmt, txRepo: txRepo}

	uc := newUC(t, repo, &noopIdemStore{})

	before := time.Now().UTC().Truncate(time.Second)
	err := uc.Execute(context.Background(), "1", providerID)
	after := time.Now().UTC().Add(time.Second)

	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	// Payment marcado como paid
	if !txRepo.markedAsPaid {
		t.Error("esperado: MarkAsPaid chamado")
	}

	// Evento registrado (idempotência interna)
	if !txRepo.registeredEvent {
		t.Error("esperado: RegisterEvent chamado")
	}

	// Subscription ativada
	if txRepo.activatedSubID != subID {
		t.Errorf("ActivateSubscriptionTx subID = %d, want %d", txRepo.activatedSubID, subID)
	}

	// period_start está no intervalo before..after
	if txRepo.activatedPeriodStart.Before(before) || txRepo.activatedPeriodStart.After(after) {
		t.Errorf("period_start fora do intervalo esperado: %v", txRepo.activatedPeriodStart)
	}

	// period_end = period_start + 30 dias
	expectedEnd := txRepo.activatedPeriodStart.AddDate(0, 0, durationDays)
	if !txRepo.activatedPeriodEnd.Equal(expectedEnd) {
		t.Errorf("period_end = %v, want %v", txRepo.activatedPeriodEnd, expectedEnd)
	}

	// Transação commitada exatamente uma vez
	if txRepo.committedCount != 1 {
		t.Errorf("Commit chamado %d vez(es), want 1", txRepo.committedCount)
	}
}

// TestMarkMPPaid_Idempotent_IdemKeyAlreadyExists verifica que reprocessar o mesmo
// webhook (idem key já salva) retorna nil sem tocar no banco.
func TestMarkMPPaid_Idempotent_IdemKeyAlreadyExists(t *testing.T) {
	const (paymentID = uint(2); subID = uint(20))
	pmt := pendingPayment(paymentID, subID, "sub_pending:20:111")
	txRepo := &mockTxRepo{payment: pmt}
	repo := &mockPaymentRepo{payment: pmt, txRepo: txRepo}

	// idem já existe → Execute deve retornar sem processar
	uc := newUC(t, repo, &noopIdemStore{exists: true})

	err := uc.Execute(context.Background(), "2", "QRC_IDEM")
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	// Nenhuma operação de banco deve ter sido feita
	if txRepo.markedAsPaid {
		t.Error("não deve chamar MarkAsPaid quando idem key já existe")
	}
	if txRepo.activatedSubID != 0 {
		t.Error("não deve ativar subscription quando idem key já existe")
	}
	if txRepo.committedCount != 0 {
		t.Error("não deve commitar quando idem key já existe")
	}
}

// TestMarkMPPaid_Idempotent_PaymentAlreadyFinal verifica que payment já em estado
// final (paid/expired) retorna nil sem reprocessar.
func TestMarkMPPaid_Idempotent_PaymentAlreadyFinal(t *testing.T) {
	const (paymentID = uint(3); subID = uint(30))
	pmt := pendingPayment(paymentID, subID, "sub_pending:30:222")
	pmt.Status = "paid" // já pago

	txRepo := &mockTxRepo{payment: pmt}
	repo := &mockPaymentRepo{payment: pmt, txRepo: txRepo}
	uc := newUC(t, repo, &noopIdemStore{})

	err := uc.Execute(context.Background(), "3", "QRC_FINAL")
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if txRepo.markedAsPaid {
		t.Error("não deve chamar MarkAsPaid para payment já paid")
	}
	if txRepo.activatedSubID != 0 {
		t.Error("não deve ativar subscription para payment já paid")
	}
	if txRepo.committedCount != 0 {
		t.Error("não deve commitar para payment já paid")
	}
}

// TestMarkMPPaid_Idempotent_SubAlreadyActive verifica que se a subscription já
// estiver active quando o webhook chega, ActivateSubscriptionTx não é chamado.
func TestMarkMPPaid_Idempotent_SubAlreadyActive(t *testing.T) {
	const (paymentID = uint(4); subID = uint(40); planID = uint(7))
	pmt := pendingPayment(paymentID, subID, "sub_pending:40:333")
	sub := pendingSubscription(subID, planID)
	sub.Status = "active" // já ativa
	plan := planWith(planID, 30)

	txRepo := &mockTxRepo{payment: pmt, sub: sub, plan: plan}
	repo := &mockPaymentRepo{payment: pmt, txRepo: txRepo}
	uc := newUC(t, repo, &noopIdemStore{})

	err := uc.Execute(context.Background(), "4", "QRC_ACTIVE")
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	// Payment é marcado como paid (evento foi recebido)
	if !txRepo.markedAsPaid {
		t.Error("payment deve ser marcado como paid mesmo se sub já estiver active")
	}

	// Mas ActivateSubscriptionTx NÃO deve ser chamado (sub não está pending_payment)
	if txRepo.activatedSubID != 0 {
		t.Errorf("ActivateSubscriptionTx não deve ser chamado para sub já active, got subID=%d", txRepo.activatedSubID)
	}
}

// TestMarkMPPaid_AppointmentPath_NotBroken verifica que o fluxo de agendamento
// continua funcionando independentemente do código de subscription.
func TestMarkMPPaid_AppointmentPath_NotBroken(t *testing.T) {
	const (paymentID = uint(5); apptID = uint(50))
	apptIDPtr := apptID
	pmt := &models.Payment{
		ID:            paymentID,
		BarbershopID:  1,
		Status:        "pending",
		TxID:          strp("mp_pay:99999"),
		AppointmentID: &apptIDPtr,
		// SubscriptionID is nil — appointment payment
	}

	appt := &models.Appointment{
		ID:     apptID,
		Status: "awaiting_payment",
	}

	txRepo := &mockTxRepo{payment: pmt, appointment: appt}
	repo := &mockPaymentRepo{payment: pmt, txRepo: txRepo}
	uc := newUC(t, repo, &noopIdemStore{})

	err := uc.Execute(context.Background(), "5", "99999")
	if err != nil {
		t.Fatalf("Execute appointment path error: %v", err)
	}

	// Payment marcado como paid
	if !txRepo.markedAsPaid {
		t.Error("appointment payment: MarkAsPaid deve ser chamado")
	}

	// Subscription NÃO deve ser ativada (não há SubscriptionID)
	if txRepo.activatedSubID != 0 {
		t.Errorf("appointment payment: ActivateSubscriptionTx não deve ser chamado, got subID=%d", txRepo.activatedSubID)
	}

	// Transação commitada
	if txRepo.committedCount != 1 {
		t.Errorf("appointment path: Commit = %d, want 1", txRepo.committedCount)
	}
}

// TestMarkMPPaid_OrderPath_NotBroken verifica que o fluxo de pedido standalone
// continua funcionando independentemente do código de subscription.
func TestMarkMPPaid_OrderPath_NotBroken(t *testing.T) {
	const (paymentID = uint(6); orderID = uint(60))
	orderIDPtr := orderID
	pmt := &models.Payment{
		ID:           paymentID,
		BarbershopID: 1,
		Status:       "pending",
		TxID:         strp("QRC_ORDER_PIX"),
		OrderID:      &orderIDPtr,
		// SubscriptionID and AppointmentID are nil — standalone order payment
	}

	order := &models.Order{
		ID:     orderID,
		Status: "pending",
	}

	txRepo := &mockTxRepo{payment: pmt, order: order}
	repo := &mockPaymentRepo{payment: pmt, txRepo: txRepo}
	uc := newUC(t, repo, &noopIdemStore{})

	err := uc.Execute(context.Background(), "6", "QRC_ORDER_PIX")
	if err != nil {
		t.Fatalf("Execute order path error: %v", err)
	}

	// Payment marcado como paid
	if !txRepo.markedAsPaid {
		t.Error("order payment: MarkAsPaid deve ser chamado")
	}

	// Subscription NÃO deve ser ativada
	if txRepo.activatedSubID != 0 {
		t.Errorf("order payment: ActivateSubscriptionTx não deve ser chamado, got subID=%d", txRepo.activatedSubID)
	}

	// Transação commitada
	if txRepo.committedCount != 1 {
		t.Errorf("order path: Commit = %d, want 1", txRepo.committedCount)
	}
}

// TestMarkMPPaid_Subscription_PeriodBoundaries verifica os limites do cálculo
// de período para planos de 1, 30 e 365 dias.
func TestMarkMPPaid_Subscription_PeriodBoundaries(t *testing.T) {
	cases := []struct {
		name         string
		durationDays int
	}{
		{"1_dia", 1},
		{"30_dias", 30},
		{"365_dias", 365},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const (payID = uint(7); subID = uint(70); planID = uint(9))
			pmt := pendingPayment(payID, subID, "sub_pending:70:888")
			sub := pendingSubscription(subID, planID)
			plan := planWith(planID, tc.durationDays)

			txRepo := &mockTxRepo{payment: pmt, sub: sub, plan: plan}
			repo := &mockPaymentRepo{payment: pmt, txRepo: txRepo}
			uc := newUC(t, repo, &noopIdemStore{})

			before := time.Now().UTC()
			if err := uc.Execute(context.Background(), "7", "QRC_PERIOD"); err != nil {
				t.Fatalf("Execute error: %v", err)
			}

			if txRepo.activatedSubID == 0 {
				t.Fatal("subscription não foi ativada")
			}

			// period_end deve ser period_start + durationDays
			want := txRepo.activatedPeriodStart.AddDate(0, 0, tc.durationDays)
			if !txRepo.activatedPeriodEnd.Equal(want) {
				t.Errorf("period_end = %v, want %v", txRepo.activatedPeriodEnd, want)
			}

			// period_start deve ser >= before
			if txRepo.activatedPeriodStart.Before(before) {
				t.Errorf("period_start %v é antes de before %v", txRepo.activatedPeriodStart, before)
			}
		})
	}
}

// TestMarkMPPaid_ExternalReferenceInvalid verifica que referências inválidas
// retornam erro sem tocar no banco.
func TestMarkMPPaid_ExternalReferenceInvalid(t *testing.T) {
	txRepo := &mockTxRepo{}
	repo := &mockPaymentRepo{txRepo: txRepo}
	uc := newUC(t, repo, &noopIdemStore{})

	err := uc.Execute(context.Background(), "", "QRC_VALID")
	if err == nil {
		t.Error("referência vazia deve retornar erro")
	}

	err = uc.Execute(context.Background(), "valid", "")
	if err == nil {
		t.Error("providerID vazio deve retornar erro")
	}

	if txRepo.markedAsPaid || txRepo.activatedSubID != 0 {
		t.Error("referência inválida não deve modificar estado")
	}
}

// ── fix de método faltante no mockPaymentRepo ────────────────────────────────────

// GetByTxID recebe 4 parâmetros mas a interface original tem 3 (context, barbershopID, txid).
// Ajuste: o mockPaymentRepo deve implementar a interface exatamente.
// (O compiler vai sinalizar se houver divergência — este comentário é documentação.)

var _ domainPayment.Repository = (*mockPaymentRepo)(nil)
var _ domainPayment.TxRepository = (*mockTxRepo)(nil)
var _ idempotency.Store = (*noopIdemStore)(nil)
var _ domainNotification.Notifier = (*noopNotifier)(nil)
var _ domainNotification.AppointmentNotifier = (*noopApptNotifier)(nil)
var _ domainTicket.Repository = (*noopTicketRepo)(nil)

// errDeadCodeElimination evita "declared and not used" em imports.
var _ = errors.New
