package repository

// Testes que verificam o comportamento de ActivateSubscriptionByID
// para cada status possível de subscription.
//
// Esses testes cobrem indiretamente a lógica de status check que
// activateSubscription (handler) usa: pending_payment ativa, active
// é idempotente, expired/cancelled retorna erro, inexistente retorna erro.
//
// Requerem DATABASE_URL — skipped automaticamente sem ele.

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// seedSubWithStatus cria uma subscription com status explícito para os testes.
func seedSubWithStatus(t *testing.T, tx *gorm.DB, barbershopID, clientID, planID uint, status string) models.Subscription {
	t.Helper()
	now := time.Now().UTC()
	sub := models.Subscription{
		BarbershopID:         barbershopID,
		ClientID:             clientID,
		PlanID:               planID,
		Status:               status,
		CurrentPeriodStart:   now.Add(-time.Hour),
		CurrentPeriodEnd:     now.Add(30 * 24 * time.Hour),
		CutsUsedInPeriod:     0,
		CutsReservedInPeriod: 0,
	}
	if err := tx.Create(&sub).Error; err != nil {
		t.Fatalf("seedSubWithStatus(%s): %v", status, err)
	}
	return sub
}

// ── testes ───────────────────────────────────────────────────────────────────────

// pending_payment → ativa com sucesso e retorna nil.
func TestActivateSubscriptionByID_PendingPayment_Ativa(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	_ = db.Transaction(func(tx *gorm.DB) error {
		bs := seedBarbershop(t, tx)
		cl := seedClient(t, tx, bs.ID)
		plan := seedPlan(t, tx, bs.ID)
		sub := seedSubWithStatus(t, tx, bs.ID, cl.ID, plan.ID, string(domain.StatusPendingPayment))

		repo := NewSubscriptionGormRepository(tx)
		err := repo.ActivateSubscriptionByID(ctx, sub.ID, now, now.AddDate(0, 0, 30))
		if err != nil {
			t.Errorf("pending_payment: esperado nil, obtido: %v", err)
		}

		var updated models.Subscription
		if err2 := tx.First(&updated, sub.ID).Error; err2 != nil {
			t.Fatalf("não encontrou subscription: %v", err2)
		}
		if updated.Status != string(domain.StatusActive) {
			t.Errorf("status esperado 'active', obtido '%s'", updated.Status)
		}

		return errors.New("rollback intencional")
	})
}

// active → retorna nil (idempotente legítimo).
func TestActivateSubscriptionByID_Active_RetornaNil(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	_ = db.Transaction(func(tx *gorm.DB) error {
		bs := seedBarbershop(t, tx)
		cl := seedClient(t, tx, bs.ID)
		plan := seedPlan(t, tx, bs.ID)
		sub := seedSubWithStatus(t, tx, bs.ID, cl.ID, plan.ID, string(domain.StatusActive))

		repo := NewSubscriptionGormRepository(tx)
		err := repo.ActivateSubscriptionByID(ctx, sub.ID, now, now.AddDate(0, 0, 30))
		if err != nil {
			t.Errorf("status active: esperado nil (idempotente), obtido: %v", err)
		}

		return errors.New("rollback intencional")
	})
}

// expired → não retorna nil silencioso — retorna ErrActiveSubscriptionNotFound.
func TestActivateSubscriptionByID_Expired_NaoRetornaNilSilencioso(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	_ = db.Transaction(func(tx *gorm.DB) error {
		bs := seedBarbershop(t, tx)
		cl := seedClient(t, tx, bs.ID)
		plan := seedPlan(t, tx, bs.ID)
		sub := seedSubWithStatus(t, tx, bs.ID, cl.ID, plan.ID, string(domain.StatusExpired))

		repo := NewSubscriptionGormRepository(tx)
		err := repo.ActivateSubscriptionByID(ctx, sub.ID, now, now.AddDate(0, 0, 30))
		if err == nil {
			t.Error("status expired: não deve retornar nil — não é idempotência legítima")
		}
		if errors.Is(err, domain.ErrActiveSubscriptionNotFound) {
			// Comportamento aceitável: retorna not found para status terminal.
			return errors.New("rollback intencional")
		}
		// Qualquer erro não-nil é aceitável aqui — o importante é não ser nil.
		t.Logf("status expired retornou erro (esperado): %v", err)
		return errors.New("rollback intencional")
	})
}

// cancelled → não retorna nil silencioso — retorna ErrActiveSubscriptionNotFound.
func TestActivateSubscriptionByID_Cancelled_NaoRetornaNilSilencioso(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	_ = db.Transaction(func(tx *gorm.DB) error {
		bs := seedBarbershop(t, tx)
		cl := seedClient(t, tx, bs.ID)
		plan := seedPlan(t, tx, bs.ID)
		sub := seedSubWithStatus(t, tx, bs.ID, cl.ID, plan.ID, string(domain.StatusCancelled))

		repo := NewSubscriptionGormRepository(tx)
		err := repo.ActivateSubscriptionByID(ctx, sub.ID, now, now.AddDate(0, 0, 30))
		if err == nil {
			t.Error("status cancelled: não deve retornar nil — não é idempotência legítima")
		}
		t.Logf("status cancelled retornou erro (esperado): %v", err)
		return errors.New("rollback intencional")
	})
}

// inexistente → retorna ErrActiveSubscriptionNotFound.
func TestActivateSubscriptionByID_Inexistente_RetornaNotFound(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	repo := NewSubscriptionGormRepository(db)
	const idInexistente = uint(888_888_888)
	err := repo.ActivateSubscriptionByID(ctx, idInexistente, now, now.AddDate(0, 0, 30))
	if !errors.Is(err, domain.ErrActiveSubscriptionNotFound) {
		t.Errorf("inexistente: esperado ErrActiveSubscriptionNotFound, obtido: %v", err)
	}
}
