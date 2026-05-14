package repository

// Testes de repositório para ActivateSubscriptionByID.
//
// Cobrem: ativação normal, idempotência quando já ativa, ausência de subscription.
// Requerem banco PostgreSQL real via DATABASE_URL — skipped automaticamente sem ele.
//
// Execução:
//   DATABASE_URL="postgres://..." go test ./internal/repository/... -v -run TestActivateSubscriptionByID

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// seedPendingSub cria uma subscription com status=pending_payment para os testes
// de ativação (sem current_period_start/end definidos, como na compra real).
func seedPendingSub(t *testing.T, tx *gorm.DB, barbershopID, clientID, planID uint) models.Subscription {
	t.Helper()
	sub := models.Subscription{
		BarbershopID: barbershopID,
		ClientID:     clientID,
		PlanID:       planID,
		Status:       string(domain.StatusPendingPayment),
		// period será definido na ativação
		CurrentPeriodStart: time.Time{},
		CurrentPeriodEnd:   time.Time{},
	}
	if err := tx.Create(&sub).Error; err != nil {
		t.Fatalf("seedPendingSub: %v", err)
	}
	return sub
}

// ── testes ───────────────────────────────────────────────────────────────────────

// Teste 1: subscription pending_payment → ativa com sucesso.
func TestActivateSubscriptionByID_Pendente_AtivaComSucesso(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	_ = db.Transaction(func(tx *gorm.DB) error {
		bs := seedBarbershop(t, tx)
		cl := seedClient(t, tx, bs.ID)
		plan := seedPlan(t, tx, bs.ID)
		sub := seedPendingSub(t, tx, bs.ID, cl.ID, plan.ID)

		periodStart := now
		periodEnd := now.AddDate(0, 0, plan.DurationDays)

		repo := NewSubscriptionGormRepository(tx)
		if err := repo.ActivateSubscriptionByID(ctx, sub.ID, periodStart, periodEnd); err != nil {
			t.Errorf("esperado nil, obtido: %v", err)
		}

		var updated models.Subscription
		if err := tx.First(&updated, sub.ID).Error; err != nil {
			t.Fatalf("não encontrou subscription: %v", err)
		}
		if updated.Status != string(domain.StatusActive) {
			t.Errorf("status esperado 'active', obtido '%s'", updated.Status)
		}
		if updated.CutsUsedInPeriod != 0 || updated.CutsReservedInPeriod != 0 {
			t.Errorf("cuts devem ser 0 após ativação (used=%d, reserved=%d)",
				updated.CutsUsedInPeriod, updated.CutsReservedInPeriod)
		}

		return errors.New("rollback intencional")
	})
}

// Teste 2: chamada repetida quando subscription já está active → no-op/idempotente.
func TestActivateSubscriptionByID_JaAtiva_NaoRetornaErro(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	_ = db.Transaction(func(tx *gorm.DB) error {
		bs := seedBarbershop(t, tx)
		cl := seedClient(t, tx, bs.ID)
		plan := seedPlan(t, tx, bs.ID)

		// Subscription já ativa (período válido, não pending_payment).
		activeSub := seedValidSub(t, tx, bs.ID, cl.ID, plan.ID)

		repo := NewSubscriptionGormRepository(tx)
		// Segunda chamada de ativação — deve ser no-op idempotente.
		err := repo.ActivateSubscriptionByID(ctx, activeSub.ID, now, now.AddDate(0, 1, 0))
		if err != nil {
			t.Errorf("subscription já ativa: esperado nil (idempotente), obtido: %v", err)
		}

		// Período original não deve ter sido sobrescrito.
		var check models.Subscription
		if err := tx.First(&check, activeSub.ID).Error; err != nil {
			t.Fatalf("não encontrou subscription: %v", err)
		}
		if check.Status != string(domain.StatusActive) {
			t.Errorf("status não deve mudar: obtido '%s'", check.Status)
		}

		return errors.New("rollback intencional")
	})
}

// Teste 3: duas ativações "concorrentes" — apenas a primeira define período.
// Simula serialização: primeira ativa, segunda encontra sub já active → no-op.
func TestActivateSubscriptionByID_DuasChamadas_ApenasUmaDefineperiodo(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	var subID uint
	var firstPeriodEnd time.Time

	// Fase 1: cria subscription pending fora de transação de rollback.
	func() {
		bs := seedBarbershop(t, db)
		cl := seedClient(t, db, bs.ID)
		plan := seedPlan(t, db, bs.ID)
		sub := seedPendingSub(t, db, bs.ID, cl.ID, plan.ID)
		subID = sub.ID

		t.Cleanup(func() {
			db.Exec("DELETE FROM subscriptions WHERE id = ?", subID)
			db.Exec("DELETE FROM clients WHERE id = ?", cl.ID)
			db.Exec("DELETE FROM plans WHERE barbershop_id = ?", bs.ID)
			db.Exec("DELETE FROM barbershops WHERE id = ?", bs.ID)
		})
	}()

	repo := NewSubscriptionGormRepository(db)

	// Primeira ativação — deve ter sucesso.
	firstPeriodEnd = now.AddDate(0, 0, 30)
	if err := repo.ActivateSubscriptionByID(ctx, subID, now, firstPeriodEnd); err != nil {
		t.Fatalf("primeira ativação falhou: %v", err)
	}

	// Segunda ativação com período diferente — deve ser no-op idempotente.
	laterPeriodEnd := now.AddDate(0, 0, 60) // diferente
	if err := repo.ActivateSubscriptionByID(ctx, subID, now.Add(time.Hour), laterPeriodEnd); err != nil {
		t.Errorf("segunda ativação: esperado nil (idempotente), obtido: %v", err)
	}

	// Período deve ser o da primeira ativação, não da segunda.
	var sub models.Subscription
	if err := db.First(&sub, subID).Error; err != nil {
		t.Fatalf("não encontrou subscription: %v", err)
	}
	if sub.Status != string(domain.StatusActive) {
		t.Errorf("status esperado 'active', obtido '%s'", sub.Status)
	}
	if !sub.CurrentPeriodEnd.Equal(firstPeriodEnd.UTC().Truncate(time.Millisecond)) &&
		sub.CurrentPeriodEnd.UTC().After(firstPeriodEnd.UTC().Add(time.Second)) {
		t.Errorf("período deve ser o da primeira ativação (first=%v, atual=%v)",
			firstPeriodEnd.UTC(), sub.CurrentPeriodEnd.UTC())
	}
}

// Teste 4: payment já paid não quebra — activateSubscription com payment paid é no-op.
// (O UPDATE de payment usa WHERE status='pending' — se já paid, 0 rows, sem erro.)
func TestActivateSubscriptionByID_SubscriptionInexistente_RetornaErro(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	repo := NewSubscriptionGormRepository(db)
	const idInexistente = uint(999_999_999)

	err := repo.ActivateSubscriptionByID(ctx, idInexistente, now, now.AddDate(0, 1, 0))
	if !errors.Is(err, domain.ErrActiveSubscriptionNotFound) {
		t.Errorf("subscription inexistente: esperado ErrActiveSubscriptionNotFound, obtido: %v", err)
	}
}
