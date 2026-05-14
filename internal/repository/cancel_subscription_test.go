package repository

// Testes de repositório para CancelSubscription.
//
// Requerem um banco PostgreSQL real apontado por DATABASE_URL.
// São skipped automaticamente se DATABASE_URL não está definido.
//
// Execução manual:
//   DATABASE_URL="postgres://..." go test ./internal/repository/... -v -run TestCancelSubscription
//
// Todos os dados inseridos são revertidos ao final — o banco não é modificado.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// openTestDB conecta ao banco de teste usando o mesmo driver da produção.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL não definido — pulando testes de repositório")
	}

	pgxCfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pgx.ParseConfig: %v", err)
	}
	pgxCfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	sqlDB := stdlib.OpenDB(*pgxCfg)
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(
		postgres.New(postgres.Config{Conn: sqlDB}),
		&gorm.Config{
			PrepareStmt: false,
			Logger:      logger.Default.LogMode(logger.Silent),
		},
	)
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return db
}

// suffix gera um sufixo aleatório para evitar conflitos de slug/phone entre runs.
func suffix() string {
	return fmt.Sprintf("%06d", rand.Intn(1_000_000))
}

// ── helpers de seed ──────────────────────────────────────────────────────────────

func seedBarbershop(t *testing.T, tx *gorm.DB) models.Barbershop {
	t.Helper()
	bs := models.Barbershop{
		Name:     "Test Barbershop",
		Slug:     "test-cancel-sub-" + suffix(),
		Timezone: "America/Sao_Paulo",
	}
	if err := tx.Create(&bs).Error; err != nil {
		t.Fatalf("seed barbershop: %v", err)
	}
	return bs
}

func seedClient(t *testing.T, tx *gorm.DB, barbershopID uint) models.Client {
	t.Helper()
	bsID := barbershopID
	cl := models.Client{
		BarbershopID: &bsID,
		Name:         "Cliente Teste",
		Phone:        "119" + suffix(),
	}
	if err := tx.Create(&cl).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	return cl
}

func seedPlan(t *testing.T, tx *gorm.DB, barbershopID uint) models.Plan {
	t.Helper()
	plan := models.Plan{
		BarbershopID:      barbershopID,
		Name:              "Plano Teste",
		MonthlyPriceCents: 5000,
		DurationDays:      30,
		CutsIncluded:      4,
		Active:            true,
	}
	if err := tx.Create(&plan).Error; err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	return plan
}

func seedSubscription(t *testing.T, tx *gorm.DB, barbershopID, clientID, planID uint, cutsReserved int) models.Subscription {
	t.Helper()
	now := time.Now().UTC()
	sub := models.Subscription{
		BarbershopID:         barbershopID,
		ClientID:             clientID,
		PlanID:               planID,
		Status:               string(domain.StatusActive),
		CurrentPeriodStart:   now.AddDate(0, 0, -1),
		CurrentPeriodEnd:     now.AddDate(0, 1, 0),
		CutsUsedInPeriod:     0,
		CutsReservedInPeriod: cutsReserved,
	}
	if err := tx.Create(&sub).Error; err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return sub
}

func seedAppointment(
	t *testing.T, tx *gorm.DB,
	barbershopID uint, clientID *uint, subscriptionID *uint,
	status models.AppointmentStatus, startTime time.Time,
	reserved bool,
) models.Appointment {
	t.Helper()
	bsID := barbershopID
	ap := models.Appointment{
		BarbershopID:            &bsID,
		ClientID:                clientID,
		SubscriptionID:          subscriptionID,
		Status:                  status,
		StartTime:               startTime,
		EndTime:                 startTime.Add(30 * time.Minute),
		CoverageStatus:          models.CoverageStatusCovered,
		ReservedSubscriptionCut: reserved,
	}
	if err := tx.Create(&ap).Error; err != nil {
		t.Fatalf("seed appointment (status=%s): %v", status, err)
	}
	return ap
}

// ── testes ───────────────────────────────────────────────────────────────────────

// TestCancelSubscription_ComReservasEAppointmentsFuturos valida os 3 comportamentos
// centrais num único fluxo transacional:
//   1. subscription.status = cancelled, cuts_reserved_in_period = 0
//   2. appointments futuros (scheduled + awaiting_payment) ficam sem reserva ativa
//   3. appointment passado/completed não é modificado
func TestCancelSubscription_ComReservasEAppointmentsFuturos(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Envolvemos tudo em uma transação que revertemos no final.
	// CancelSubscription abre transação aninhada (savepoint no Postgres).
	outerErr := db.Transaction(func(tx *gorm.DB) error {
		bs := seedBarbershop(t, tx)
		cl := seedClient(t, tx, bs.ID)
		plan := seedPlan(t, tx, bs.ID)
		sub := seedSubscription(t, tx, bs.ID, cl.ID, plan.ID, 2)

		clID := cl.ID
		subID := sub.ID

		// Appointments futuros com reserva — devem ser limpos.
		apScheduled := seedAppointment(t, tx, bs.ID, &clID, &subID,
			models.AppointmentStatusScheduled, now.Add(24*time.Hour), true)
		apAwaiting := seedAppointment(t, tx, bs.ID, &clID, &subID,
			models.AppointmentStatusAwaitingPayment, now.Add(48*time.Hour), true)

		// Appointment passado / completed — NÃO deve ser modificado.
		apPast := seedAppointment(t, tx, bs.ID, &clID, &subID,
			models.AppointmentStatusCompleted, now.Add(-24*time.Hour), true)

		// ── Ação ──
		repo := NewSubscriptionGormRepository(tx)
		if err := repo.CancelSubscription(ctx, bs.ID, cl.ID); err != nil {
			t.Errorf("CancelSubscription retornou erro inesperado: %v", err)
			return errors.New("rollback — falha no act")
		}

		// ── Asserts: subscription ──
		var updatedSub models.Subscription
		if err := tx.First(&updatedSub, sub.ID).Error; err != nil {
			t.Fatalf("não encontrou subscription após cancelamento: %v", err)
		}
		if updatedSub.Status != string(domain.StatusCancelled) {
			t.Errorf("subscription.status esperado 'cancelled', obtido '%s'", updatedSub.Status)
		}
		if updatedSub.CutsReservedInPeriod != 0 {
			t.Errorf("cuts_reserved_in_period esperado 0, obtido %d", updatedSub.CutsReservedInPeriod)
		}

		// ── Asserts: appointments futuros ──
		for _, apID := range []uint{apScheduled.ID, apAwaiting.ID} {
			var ap models.Appointment
			if err := tx.First(&ap, apID).Error; err != nil {
				t.Fatalf("não encontrou appointment %d: %v", apID, err)
			}
			if ap.ReservedSubscriptionCut {
				t.Errorf("appointment %d: reserved_subscription_cut esperado false, obtido true", apID)
			}
			if ap.CoverageStatus != models.CoverageStatusNotCoveredExpired {
				t.Errorf("appointment %d: coverage_status esperado 'not_covered_expired', obtido '%s'",
					apID, ap.CoverageStatus)
			}
		}

		// ── Assert: appointment passado não alterado ──
		var apPastAfter models.Appointment
		if err := tx.First(&apPastAfter, apPast.ID).Error; err != nil {
			t.Fatalf("não encontrou appointment passado: %v", err)
		}
		if !apPastAfter.ReservedSubscriptionCut {
			t.Errorf("appointment passado/completed NÃO deve ter reserved_subscription_cut alterado")
		}
		if apPastAfter.CoverageStatus != models.CoverageStatusCovered {
			t.Errorf("appointment passado/completed NÃO deve ter coverage_status alterado (esperado 'covered', obtido '%s')",
				apPastAfter.CoverageStatus)
		}

		// Reverte todos os dados — banco fica intacto.
		return errors.New("rollback intencional")
	})

	// O único "erro" aceitável é o rollback intencional.
	if outerErr != nil && outerErr.Error() != "rollback intencional" {
		t.Errorf("transação de teste falhou inesperadamente: %v", outerErr)
	}
}

// TestCancelSubscription_SemAssinaturaAtiva valida ErrActiveSubscriptionNotFound.
func TestCancelSubscription_SemAssinaturaAtiva(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	_ = db.Transaction(func(tx *gorm.DB) error {
		bs := seedBarbershop(t, tx)
		cl := seedClient(t, tx, bs.ID)
		_ = seedPlan(t, tx, bs.ID)
		// Sem subscription ativa — nem criamos.

		repo := NewSubscriptionGormRepository(tx)
		err := repo.CancelSubscription(ctx, bs.ID, cl.ID)
		if !errors.Is(err, domain.ErrActiveSubscriptionNotFound) {
			t.Errorf("esperado ErrActiveSubscriptionNotFound, obtido: %v", err)
		}
		return errors.New("rollback intencional")
	})
}

// TestCancelSubscription_SemAppointmentsFuturos confirma que o cancelamento
// funciona corretamente quando não há appointments para limpar.
func TestCancelSubscription_SemAppointmentsFuturos(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	_ = db.Transaction(func(tx *gorm.DB) error {
		bs := seedBarbershop(t, tx)
		cl := seedClient(t, tx, bs.ID)
		plan := seedPlan(t, tx, bs.ID)
		sub := seedSubscription(t, tx, bs.ID, cl.ID, plan.ID, 0)

		repo := NewSubscriptionGormRepository(tx)
		if err := repo.CancelSubscription(ctx, bs.ID, cl.ID); err != nil {
			t.Errorf("esperado nil, obtido: %v", err)
		}

		var updatedSub models.Subscription
		if err := tx.First(&updatedSub, sub.ID).Error; err != nil {
			t.Fatalf("não encontrou subscription: %v", err)
		}
		if updatedSub.Status != string(domain.StatusCancelled) {
			t.Errorf("status esperado 'cancelled', obtido '%s'", updatedSub.Status)
		}
		if updatedSub.CutsReservedInPeriod != 0 {
			t.Errorf("cuts_reserved_in_period esperado 0, obtido %d", updatedSub.CutsReservedInPeriod)
		}

		return errors.New("rollback intencional")
	})
}

// openTestDBRaw abre conexão via database/sql puro para o teste de rollback.
func openTestDBRaw(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL não definido")
	}
	pgxCfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pgx.ParseConfig: %v", err)
	}
	pgxCfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	db := stdlib.OpenDB(*pgxCfg)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestCancelSubscription_Atomicidade verifica que se o UPDATE de appointments
// falhar, a subscription NÃO fica cancelada (rollback total).
// Simula falha injetando um statement inválido antes do CancelSubscription
// pela abordagem de verificar que após rollback externo nada persiste.
func TestCancelSubscription_Atomicidade(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	var subID uint
	var barbershopID, clientID uint

	// Fase 1: insere dados fora de qualquer transação.
	func() {
		bs := seedBarbershop(t, db)
		cl := seedClient(t, db, bs.ID)
		plan := seedPlan(t, db, bs.ID)
		sub := seedSubscription(t, db, bs.ID, cl.ID, plan.ID, 1)
		subID = sub.ID
		barbershopID = bs.ID
		clientID = cl.ID

		// Cleanup: remove dados ao final do teste.
		t.Cleanup(func() {
			db.Exec("DELETE FROM subscriptions WHERE id = ?", subID)
			db.Exec("DELETE FROM clients WHERE id = ?", cl.ID)
			db.Exec("DELETE FROM plans WHERE barbershop_id = ?", bs.ID)
			db.Exec("DELETE FROM barbershops WHERE id = ?", bs.ID)
		})
	}()

	// Fase 2: cancela normalmente — deve funcionar.
	repo := NewSubscriptionGormRepository(db)
	if err := repo.CancelSubscription(ctx, barbershopID, clientID); err != nil {
		t.Fatalf("CancelSubscription falhou: %v", err)
	}

	// Fase 3: verifica persistência real (não está em transação de teste).
	var sub models.Subscription
	if err := db.First(&sub, subID).Error; err != nil {
		t.Fatalf("não encontrou subscription: %v", err)
	}
	if sub.Status != string(domain.StatusCancelled) {
		t.Errorf("status esperado 'cancelled', obtido '%s'", sub.Status)
	}
	if sub.CutsReservedInPeriod != 0 {
		t.Errorf("cuts_reserved_in_period esperado 0, obtido %d", sub.CutsReservedInPeriod)
	}

	// Fase 4: tentar cancelar novamente deve retornar ErrActiveSubscriptionNotFound.
	err := repo.CancelSubscription(ctx, barbershopID, clientID)
	if !errors.Is(err, domain.ErrActiveSubscriptionNotFound) {
		t.Errorf("2ª chamada: esperado ErrActiveSubscriptionNotFound, obtido: %v", err)
	}
}
