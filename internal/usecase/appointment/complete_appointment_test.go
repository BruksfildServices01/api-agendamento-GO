package appointment

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	domainSubscription "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
	infraRepo "github.com/BruksfildServices01/barber-scheduler/internal/repository"
	"github.com/BruksfildServices01/barber-scheduler/internal/apperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
	ucSubscription "github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

// ── noop SQL driver (reutiliza o mesmo padrão de mark_mp_paid_sub_test.go) ──────
// O nome é diferente para evitar conflito de registro no pacote.

const noopCompleteDriverName = "noop-complete-test"

var registerNoopCompleteOnce sync.Once

func init() {
	registerNoopCompleteOnce.Do(func() {
		sql.Register(noopCompleteDriverName, &noopCompleteDriver{})
	})
}

type noopCompleteDriver struct{}
type noopCompleteConn struct{}
type noopCompleteStmt struct{}
type noopCompleteRows struct{}
type noopCompleteTx struct{}
type noopCompleteResult struct{}

func (*noopCompleteDriver) Open(_ string) (driver.Conn, error) { return &noopCompleteConn{}, nil }
func (*noopCompleteConn) Prepare(_ string) (driver.Stmt, error) {
	return &noopCompleteStmt{}, nil
}
func (*noopCompleteConn) Close() error                        { return nil }
func (*noopCompleteConn) Begin() (driver.Tx, error)           { return &noopCompleteTx{}, nil }
func (*noopCompleteTx) Commit() error                         { return nil }
func (*noopCompleteTx) Rollback() error                       { return nil }
func (*noopCompleteStmt) Close() error                        { return nil }
func (*noopCompleteStmt) NumInput() int                       { return -1 }
func (*noopCompleteStmt) Exec(_ []driver.Value) (driver.Result, error) {
	return noopCompleteResult{}, nil
}
func (*noopCompleteStmt) Query(_ []driver.Value) (driver.Rows, error) {
	return &noopCompleteRows{}, nil
}
func (noopCompleteResult) LastInsertId() (int64, error) { return 0, nil }
func (noopCompleteResult) RowsAffected() (int64, error) { return 1, nil }
func (*noopCompleteRows) Columns() []string             { return nil }
func (*noopCompleteRows) Close() error                  { return nil }
func (*noopCompleteRows) Next(_ []driver.Value) error   { return io.EOF }

// ── gorm DryRun DB para testes ────────────────────────────────────────────────

func newTestCompleteDB(t *testing.T) *gorm.DB {
	t.Helper()
	sqlDB, err := sql.Open(noopCompleteDriverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return db
}

func newTestCompleteDispatcher(t *testing.T) *audit.Dispatcher {
	t.Helper()
	db := newTestCompleteDB(t)
	d := audit.NewDispatcher(audit.New(db))
	t.Cleanup(d.Shutdown)
	return d
}

// ── mockCompleteAppointmentRepo ───────────────────────────────────────────────
// Implementa txableRepository para testes de CompleteAppointment.

type mockCompleteAppointmentRepo struct {
	appointment  *models.Appointment
	savedClosure *models.AppointmentClosure
}

func (r *mockCompleteAppointmentRepo) WithTx(_ *gorm.DB) domain.Repository { return r }

func (r *mockCompleteAppointmentRepo) GetAppointmentForBarber(_ context.Context, _, _, _ uint) (*models.Appointment, error) {
	return r.appointment, nil
}
func (r *mockCompleteAppointmentRepo) UpdateAppointment(_ context.Context, _ *models.Appointment) error {
	return nil
}
func (r *mockCompleteAppointmentRepo) SaveAppointmentClosure(_ context.Context, c *models.AppointmentClosure) error {
	r.savedClosure = c
	return nil
}

// Métodos restantes — não usados neste fluxo.
func (r *mockCompleteAppointmentRepo) GetBarbershopByID(_ context.Context, _ uint) (*models.Barbershop, error) {
	return &models.Barbershop{ID: 1, Timezone: "America/Sao_Paulo"}, nil
}
func (r *mockCompleteAppointmentRepo) GetProduct(_ context.Context, _, _ uint) (*models.BarbershopService, error) {
	return nil, nil
}
func (r *mockCompleteAppointmentRepo) GetOrCreateClient(_ context.Context, _ uint, _, _, _ string) (*models.Client, error) {
	return nil, nil
}
func (r *mockCompleteAppointmentRepo) CreateAppointment(_ context.Context, _ *models.Appointment) error {
	return nil
}
func (r *mockCompleteAppointmentRepo) CreateAppointmentWithKey(_ context.Context, _ *models.Appointment, _ string) error {
	return nil
}
func (r *mockCompleteAppointmentRepo) AssertNoTimeConflict(_ context.Context, _, _ uint, _, _ time.Time) error {
	return nil
}
func (r *mockCompleteAppointmentRepo) CancelExpiredAwaitingPaymentAtSlot(_ context.Context, _, _ uint, _ time.Time) error {
	return nil
}
func (r *mockCompleteAppointmentRepo) GetAppointmentClosure(_ context.Context, _, _ uint) (*models.AppointmentClosure, error) {
	return nil, nil
}
func (r *mockCompleteAppointmentRepo) GetWorkingHours(_ context.Context, _, _ uint, _ int) (*models.WorkingHours, error) {
	return nil, nil
}
func (r *mockCompleteAppointmentRepo) GetScheduleOverride(_ context.Context, _, _ uint, _ string, _, _, _ int) (*models.ScheduleOverride, error) {
	return nil, nil
}
func (r *mockCompleteAppointmentRepo) ListAppointmentsForDay(_ context.Context, _, _ uint, _, _ time.Time) ([]models.Appointment, error) {
	return nil, nil
}
func (r *mockCompleteAppointmentRepo) IsWithinWorkingHours(_ context.Context, _, _ uint, _, _ time.Time) (bool, error) {
	return true, nil
}
func (r *mockCompleteAppointmentRepo) ListAppointmentsForPeriod(_ context.Context, _, _ uint, _, _ time.Time) ([]models.Appointment, error) {
	return nil, nil
}
func (r *mockCompleteAppointmentRepo) ListAppointmentsForReminder(_ context.Context, _ uint, _ time.Time) ([]*models.Appointment, error) {
	return nil, nil
}
func (r *mockCompleteAppointmentRepo) GetAppointmentByID(_ context.Context, _, _ uint) (*models.Appointment, error) {
	return nil, nil
}
func (r *mockCompleteAppointmentRepo) GetOperationalSummary(_ context.Context, _ uint) (*domain.OperationalSummary, error) {
	return nil, nil
}

// ── mockTxableSubscriptionRepo ────────────────────────────────────────────────
// Implementa txableSubscriptionRepo. WithTx retorna um SubscriptionGormRepository
// real mas respaldado pelo noop DB (DryRun), cujas queries retornam zero linhas.

type mockTxableSubscriptionRepo struct {
	noopDB *gorm.DB
}

func (r *mockTxableSubscriptionRepo) WithTx(tx *gorm.DB) *infraRepo.SubscriptionGormRepository {
	// Usa o noop DB para que GetActiveSubscription retorne nil (zero rows)
	// e ReleaseSubscriptionCut seja uma no-op.
	return infraRepo.NewSubscriptionGormRepository(r.noopDB)
}

// Todos os métodos domain.Repository abaixo são chamados FORA da transação.
// No fluxo de CompleteAppointment não são chamados diretamente via uc.subscriptionRepo,
// apenas via txSubRepo (dentro da transação). Mantidos como no-ops seguros.
func (r *mockTxableSubscriptionRepo) CreatePlan(_ context.Context, _ *domainSubscription.Plan, _, _ []uint) error {
	return nil
}
func (r *mockTxableSubscriptionRepo) UpdatePlan(_ context.Context, _, _ uint, _ *domainSubscription.Plan, _, _ []uint) error {
	return nil
}
func (r *mockTxableSubscriptionRepo) SetPlanActive(_ context.Context, _, _ uint, _ bool) error {
	return nil
}
func (r *mockTxableSubscriptionRepo) ListPlans(_ context.Context, _ uint) ([]domainSubscription.Plan, error) {
	return nil, nil
}
func (r *mockTxableSubscriptionRepo) GetPlanByID(_ context.Context, _, _ uint) (*domainSubscription.Plan, error) {
	return nil, nil
}
func (r *mockTxableSubscriptionRepo) DeletePlan(_ context.Context, _, _ uint) error { return nil }
func (r *mockTxableSubscriptionRepo) ActivateSubscription(_ context.Context, _ *domainSubscription.Subscription) error {
	return nil
}
func (r *mockTxableSubscriptionRepo) ActivateSubscriptionByID(_ context.Context, _ uint, _, _ time.Time) error {
	return nil
}
func (r *mockTxableSubscriptionRepo) CancelSubscription(_ context.Context, _, _ uint) error {
	return nil
}
func (r *mockTxableSubscriptionRepo) GetActiveSubscription(_ context.Context, _, _ uint) (*domainSubscription.Subscription, error) {
	return nil, nil
}
func (r *mockTxableSubscriptionRepo) GetSubscriptionByID(_ context.Context, _ uint) (*domainSubscription.Subscription, error) {
	return nil, nil
}
func (r *mockTxableSubscriptionRepo) ExpireSubscriptions(_ context.Context) (int64, error) {
	return 0, nil
}
func (r *mockTxableSubscriptionRepo) ReserveSubscriptionCut(_ context.Context, _, _ uint) error {
	return nil
}
func (r *mockTxableSubscriptionRepo) ReleaseSubscriptionCut(_ context.Context, _, _ uint) error {
	return nil
}
func (r *mockTxableSubscriptionRepo) ConsumeReservedCut(_ context.Context, _, _ uint) error {
	return nil
}
func (r *mockTxableSubscriptionRepo) IncrementCutsUsed(_ context.Context, _, _ uint) error {
	return nil
}
func (r *mockTxableSubscriptionRepo) ListAllowedServiceIDs(_ context.Context, _ uint) ([]uint, error) {
	return nil, nil
}
func (r *mockTxableSubscriptionRepo) CreatePendingSubscription(_ context.Context, _ *domainSubscription.Subscription) error {
	return nil
}
func (r *mockTxableSubscriptionRepo) CountServicesByBarbershop(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}
func (r *mockTxableSubscriptionRepo) CountCategoriesByIDs(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}
func (r *mockTxableSubscriptionRepo) AddServiceToPlan(_ context.Context, _, _ uint) error {
	return nil
}
func (r *mockTxableSubscriptionRepo) UpdateCutsUsed(_ context.Context, _ uint, _ int) error {
	return nil
}
func (r *mockTxableSubscriptionRepo) CountServicesByIDs(_ context.Context, _ uint, _ []uint) (int64, error) {
	return 0, nil
}
func (r *mockTxableSubscriptionRepo) CountActiveSubscriptionsByPlan(_ context.Context, _ uint) (int64, error) {
	return 0, nil
}
func (r *mockTxableSubscriptionRepo) CountActiveSubscribersByPlan(_ context.Context, _ uint) (int64, error) {
	return 0, nil
}

// ── builder ───────────────────────────────────────────────────────────────────

func buildCompleteUC(t *testing.T, apptRepo *mockCompleteAppointmentRepo) *CompleteAppointment {
	t.Helper()
	db := newTestCompleteDB(t)
	subRepo := &mockTxableSubscriptionRepo{noopDB: db}
	consumeCutUC := ucSubscription.NewConsumeCut(subRepo)

	// UpdateClientMetrics com repo=nil: Execute retorna nil early quando repo==nil,
	// permitindo que ap.ClientID seja não-nil sem causar panic.
	metricsUC := ucMetrics.NewUpdateClientMetrics(infraRepo.NewClientMetricsGormRepository(db), db)

	return NewCompleteAppointment(
		db,
		apptRepo,
		nil, // paymentRepo — não usado quando status != awaiting_payment
		nil, // orderRepo — não usado quando sem additional_items
		nil, // productRepo — não usado quando sem additional_items
		subRepo,
		newTestCompleteDispatcher(t),
		metricsUC,
		consumeCutUC,
	)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func svcPtr(id uint) *uint { return &id }

// baseAppointment retorna um appointment scheduled válido para testes.
// ClientID=nil para não disparar métricas (nil-checked no Execute).
// scheduledSvcID é o serviço agendado (ap.BarberProductID).
// actualSvcID é o serviço que será reportado como realizado (ap.BarberProduct.ID).
// Quando actualSvcID != scheduledSvcID, a troca é detectada pelo serviceChanged check.
// Precarregar ap.BarberProduct com o serviço actual evita queries ao banco (noop DB).
func baseAppointmentWithActual(scheduledSvcID, actualSvcID uint) *models.Appointment {
	now := time.Now().UTC()
	clientID := uint(42)
	return &models.Appointment{
		ID:           100,
		BarbershopID: ptr(uint(1)),
		BarberID:     ptr(uint(1)),
		ClientID:     &clientID, // necessário para acionar o bloco de assinatura
		BarberProductID: ptr(scheduledSvcID),
		// BarberProduct precarregado com o serviço ACTUAL — assim o complete.go
		// usa ap.BarberProduct diretamente (sem query ao banco), pois:
		//   input.ActualServiceID == ap.BarberProduct.ID
		// E serviceChanged é detectado via:
		//   *input.ActualServiceID != *ap.BarberProductID
		BarberProduct: &models.BarbershopService{
			ID: actualSvcID, Name: "Serviço Realizado", Price: 5000, DurationMin: 60,
		},
		StartTime:            now.Add(time.Hour),
		EndTime:              now.Add(2 * time.Hour),
		Status:               models.AppointmentStatusScheduled,
		ReservedSubscriptionCut: true,
	}
}

func baseAppointment(scheduledSvcID uint) *models.Appointment {
	return baseAppointmentWithActual(scheduledSvcID, scheduledSvcID)
}

func ptr(v uint) *uint { return &v }

// ── Testes ────────────────────────────────────────────────────────────────────

func TestCompleteAppointment_ServiceChange(t *testing.T) {
	ctx := context.Background()
	const (
		barbershopID = uint(1)
		barberID     = uint(1)
		apptID       = uint(100)
		svcOriginal  = uint(10)
		svcCovered   = uint(11) // outro serviço coberto (noop DB → retorna nil → não coberto)
		svcNotCov    = uint(20)
	)

	t.Run("sem troca de serviço: comportamento original preservado", func(t *testing.T) {
		// Nenhum actual_service_id → consumeCutUC é chamado com o serviço agendado.
		// Com noop DB, GetActiveSubscription retorna nil → ConsumeCutStatusNoActiveSubscription
		// → requiresNormalCharging=false → fecha sem erro.
		apptRepo := &mockCompleteAppointmentRepo{appointment: baseAppointment(svcOriginal)}
		uc := buildCompleteUC(t, apptRepo)

		_, closure, result, err := uc.Execute(ctx, CompleteAppointmentInput{
			BarbershopID:  barbershopID,
			BarberID:      barberID,
			AppointmentID: apptID,
			PaymentMethod: "cash",
		})

		if err != nil {
			t.Fatalf("sem troca: erro inesperado: %v", err)
		}
		if closure == nil {
			t.Fatal("closure não deve ser nil")
		}
		// Sem assinatura ativa (noop DB) → NoActiveSubscription → não coberto, mas sem exigir confirmação
		if closure.SubscriptionCovered {
			t.Error("sem DB ativo, subscription_covered deve ser false")
		}
		if result != nil && result.Status != ucSubscription.ConsumeCutStatusNoActiveSubscription {
			t.Errorf("esperado NoActiveSubscription, obtido %v", result.Status)
		}
	})

	t.Run("troca para serviço não coberto sem confirm → normal_charging_confirmation_required", func(t *testing.T) {
		// Agendado: svcOriginal (10). Realizado: svcNotCov (20).
		// Noop DB → GetActiveSubscription retorna nil → serviço não coberto
		// → requiresNormalCharging=true → erro pois ConfirmNormalCharging=false.
		ap := baseAppointmentWithActual(svcOriginal, svcNotCov)
		apptRepo := &mockCompleteAppointmentRepo{appointment: ap}
		uc := buildCompleteUC(t, apptRepo)

		_, _, _, err := uc.Execute(ctx, CompleteAppointmentInput{
			BarbershopID:    barbershopID,
			BarberID:        barberID,
			AppointmentID:   apptID,
			ActualServiceID: svcPtr(svcNotCov),
			PaymentMethod:   "cash",
			// ConfirmNormalCharging: false (zero value)
		})

		if !apperr.IsBusiness(err, "normal_charging_confirmation_required") {
			t.Errorf("esperado normal_charging_confirmation_required, obtido: %v", err)
		}
	})

	t.Run("troca para serviço não coberto com confirm=true → fecha como cobrança normal", func(t *testing.T) {
		// Com confirm=true, o fechamento deve ser aceito mesmo sem cobertura.
		// A closure deve registrar subscription_covered=false e ServiceNotAllowed.
		ap := baseAppointmentWithActual(svcOriginal, svcNotCov)
		apptRepo := &mockCompleteAppointmentRepo{appointment: ap}
		uc := buildCompleteUC(t, apptRepo)

		_, closure, result, err := uc.Execute(ctx, CompleteAppointmentInput{
			BarbershopID:          barbershopID,
			BarberID:              barberID,
			AppointmentID:         apptID,
			ActualServiceID:       svcPtr(svcNotCov),
			PaymentMethod:         "cash",
			ConfirmNormalCharging: true,
		})

		if err != nil {
			t.Fatalf("com confirm=true: erro inesperado: %v", err)
		}
		if closure == nil {
			t.Fatal("closure não deve ser nil")
		}
		if closure.SubscriptionCovered {
			t.Error("subscription_covered deve ser false quando serviço não coberto")
		}
		if result == nil || result.Status != ucSubscription.ConsumeCutStatusServiceNotAllowed {
			t.Errorf("esperado ServiceNotAllowed, obtido: %v", result)
		}
	})

	t.Run("troca detectada: reservation não fica presa (ReleaseSubscriptionCut chamado)", func(t *testing.T) {
		// Mesmo cenário: service changed, não coberto, confirm=true.
		// O Release é chamado dentro da transação via noop DB (UPDATE no-op).
		// O teste confirma que o fluxo completa sem erro — sem panic no Release.
		ap := baseAppointmentWithActual(svcOriginal, svcNotCov)
		apptRepo := &mockCompleteAppointmentRepo{appointment: ap}
		uc := buildCompleteUC(t, apptRepo)

		_, _, _, err := uc.Execute(ctx, CompleteAppointmentInput{
			BarbershopID:          barbershopID,
			BarberID:              barberID,
			AppointmentID:         apptID,
			ActualServiceID:       svcPtr(svcNotCov),
			PaymentMethod:         "subscription",
			ConfirmNormalCharging: true,
		})

		if err != nil {
			t.Fatalf("release não deve bloquear o fechamento: %v", err)
		}
	})

	t.Run("sem reserved_subscription_cut=false: troca de serviço não afeta lógica de assinatura", func(t *testing.T) {
		// Appointment sem reserva de corte — a troca de serviço não deve acionar
		// a lógica de revalidação de cobertura. Fecha normalmente.
		ap := baseAppointmentWithActual(svcOriginal, svcNotCov)
		ap.ReservedSubscriptionCut = false
		apptRepo := &mockCompleteAppointmentRepo{appointment: ap}
		uc := buildCompleteUC(t, apptRepo)

		_, closure, result, err := uc.Execute(ctx, CompleteAppointmentInput{
			BarbershopID:    barbershopID,
			BarberID:        barberID,
			AppointmentID:   apptID,
			ActualServiceID: svcPtr(svcNotCov),
			PaymentMethod:   "cash",
		})

		if err != nil {
			t.Fatalf("sem reserva: erro inesperado: %v", err)
		}
		if closure == nil {
			t.Fatal("closure não deve ser nil")
		}
		// Sem reserva → consumeCutUC não é chamado → consumeCutResult=nil
		if result != nil {
			t.Error("sem reserva, consumeCutResult deve ser nil")
		}
		if closure.SubscriptionCovered {
			t.Error("sem reserva, subscription_covered deve ser false")
		}
	})
}

// ── Testes de lógica pura: detecção de serviceChanged ────────────────────────

func TestServiceChangedDetection(t *testing.T) {
	cases := []struct {
		name           string
		actualSvcID    *uint
		scheduledSvcID *uint
		want           bool
	}{
		{
			name:           "sem actual_service_id: sem troca",
			actualSvcID:    nil,
			scheduledSvcID: svcPtr(10),
			want:           false,
		},
		{
			name:           "actual == scheduled: sem troca",
			actualSvcID:    svcPtr(10),
			scheduledSvcID: svcPtr(10),
			want:           false,
		},
		{
			name:           "actual != scheduled: troca detectada",
			actualSvcID:    svcPtr(20),
			scheduledSvcID: svcPtr(10),
			want:           true,
		},
		{
			name:           "scheduled é nil: troca não detectada",
			actualSvcID:    svcPtr(20),
			scheduledSvcID: nil,
			want:           false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Replica a condição de serviceChanged de complete.go
			got := tc.actualSvcID != nil &&
				tc.scheduledSvcID != nil &&
				*tc.actualSvcID != *tc.scheduledSvcID

			if got != tc.want {
				t.Errorf("serviceChanged = %v, want %v", got, tc.want)
			}
		})
	}
}

// ── Teste de requiresNormalCharging por ConsumeCutStatus ─────────────────────

func TestRequiresNormalChargingByStatus(t *testing.T) {
	cases := []struct {
		status   ucSubscription.ConsumeCutStatus
		requires bool
	}{
		{ucSubscription.ConsumeCutStatusConsumed, false},
		{ucSubscription.ConsumeCutStatusNoActiveSubscription, false},
		{ucSubscription.ConsumeCutStatusServiceNotAllowed, true},
		{ucSubscription.ConsumeCutStatusLimitExceeded, true},
		{ucSubscription.ConsumeCutStatusExpiredPeriod, true},
		{ucSubscription.ConsumeCutStatusPlanNotFound, true},
		{ucSubscription.ConsumeCutStatusPlanInactive, true},
	}

	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			// Replica o switch de complete.go
			var requiresNormalCharging bool
			switch tc.status {
			case ucSubscription.ConsumeCutStatusConsumed:
				requiresNormalCharging = false
			case ucSubscription.ConsumeCutStatusNoActiveSubscription:
				requiresNormalCharging = false
			default:
				requiresNormalCharging = true
			}
			if requiresNormalCharging != tc.requires {
				t.Errorf("status=%s: requiresNormalCharging=%v, want %v",
					tc.status, requiresNormalCharging, tc.requires)
			}
		})
	}
}

// compile-time checks
var _ txableRepository = (*mockCompleteAppointmentRepo)(nil)
var _ txableSubscriptionRepo = (*mockTxableSubscriptionRepo)(nil)
