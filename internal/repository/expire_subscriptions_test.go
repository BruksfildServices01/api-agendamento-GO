package repository

// Testes de repositório para ExpireSubscriptions.
//
// Requerem banco PostgreSQL real via DATABASE_URL — skipped automaticamente sem ele.
//
// Execução manual:
//   DATABASE_URL="postgres://..." go test ./internal/repository/... -v -run TestExpireSubscriptions
//
// Os helpers de setup (openTestDB, seedBarbershop, seedClient, seedPlan,
// seedSubscription, seedAppointment) estão em cancel_subscription_test.go
// (mesmo pacote repository).

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/subscription"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// seedExpiredSub cria uma subscription active mas com current_period_end no passado.
// O job vai expirar essa linha.
func seedExpiredSub(t *testing.T, tx *gorm.DB, barbershopID, clientID, planID uint, cutsReserved int) models.Subscription {
	t.Helper()
	now := time.Now().UTC()
	sub := models.Subscription{
		BarbershopID:         barbershopID,
		ClientID:             clientID,
		PlanID:               planID,
		Status:               string(domain.StatusActive),
		CurrentPeriodStart:   now.AddDate(0, -2, 0),
		CurrentPeriodEnd:     now.Add(-time.Hour), // expirou 1h atrás
		CutsUsedInPeriod:     0,
		CutsReservedInPeriod: cutsReserved,
	}
	if err := tx.Create(&sub).Error; err != nil {
		t.Fatalf("seedExpiredSub: %v", err)
	}
	return sub
}

// seedValidSub cria uma subscription active com período ainda válido.
func seedValidSub(t *testing.T, tx *gorm.DB, barbershopID, clientID, planID uint) models.Subscription {
	t.Helper()
	now := time.Now().UTC()
	sub := models.Subscription{
		BarbershopID:         barbershopID,
		ClientID:             clientID,
		PlanID:               planID,
		Status:               string(domain.StatusActive),
		CurrentPeriodStart:   now.AddDate(0, 0, -1),
		CurrentPeriodEnd:     now.AddDate(0, 1, 0), // válida por mais 1 mês
		CutsUsedInPeriod:     0,
		CutsReservedInPeriod: 1,
	}
	if err := tx.Create(&sub).Error; err != nil {
		t.Fatalf("seedValidSub: %v", err)
	}
	return sub
}

// ── testes ───────────────────────────────────────────────────────────────────────

// TestExpireSubscriptions_FluxoCompleto valida todos os cenários obrigatórios:
//   - subscription active expirada → status='expired', cuts_reserved_in_period=0
//   - appointment futuro scheduled com reserva → limpo
//   - appointment futuro awaiting_payment com reserva → limpo
//   - appointment passado/completed com reserva → intacto
//   - appointment cancelled → intacto
//   - subscription válida → não é expirada, seus dados não são alterados
func TestExpireSubscriptions_FluxoCompleto(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	outerErr := db.Transaction(func(tx *gorm.DB) error {
		bs := seedBarbershop(t, tx)
		cl := seedClient(t, tx, bs.ID)
		plan := seedPlan(t, tx, bs.ID)

		// Subscription expirada com 2 cortes reservados.
		expiredSub := seedExpiredSub(t, tx, bs.ID, cl.ID, plan.ID, 2)
		expiredSubID := expiredSub.ID
		clID := cl.ID

		// Subscription ainda válida — não deve ser tocada.
		validSub := seedValidSub(t, tx, bs.ID, cl.ID, plan.ID)

		// Appointments futuros vinculados à subscription expirada.
		apScheduled := seedAppointment(t, tx, bs.ID, &clID, &expiredSubID,
			models.AppointmentStatusScheduled, now.Add(24*time.Hour), true)
		apAwaiting := seedAppointment(t, tx, bs.ID, &clID, &expiredSubID,
			models.AppointmentStatusAwaitingPayment, now.Add(48*time.Hour), true)

		// Appointment passado/completed — NÃO deve ser alterado.
		apCompleted := seedAppointment(t, tx, bs.ID, &clID, &expiredSubID,
			models.AppointmentStatusCompleted, now.Add(-24*time.Hour), true)

		// Appointment cancelado — NÃO deve ser alterado.
		apCancelled := seedAppointment(t, tx, bs.ID, &clID, &expiredSubID,
			models.AppointmentStatusCancelled, now.Add(24*time.Hour), true)

		// ── Ação ──
		repo := NewSubscriptionGormRepository(tx)
		n, err := repo.ExpireSubscriptions(ctx)
		if err != nil {
			t.Errorf("ExpireSubscriptions retornou erro: %v", err)
			return errors.New("rollback — falha no act")
		}
		if n < 1 {
			t.Errorf("esperado pelo menos 1 subscription expirada, obtido %d", n)
		}

		// ── Assert: subscription expirada ──
		var updatedExpired models.Subscription
		if err := tx.First(&updatedExpired, expiredSub.ID).Error; err != nil {
			t.Fatalf("não encontrou subscription expirada: %v", err)
		}
		if updatedExpired.Status != string(domain.StatusExpired) {
			t.Errorf("status esperado 'expired', obtido '%s'", updatedExpired.Status)
		}
		if updatedExpired.CutsReservedInPeriod != 0 {
			t.Errorf("cuts_reserved_in_period esperado 0, obtido %d", updatedExpired.CutsReservedInPeriod)
		}

		// ── Assert: subscription válida intacta ──
		var updatedValid models.Subscription
		if err := tx.First(&updatedValid, validSub.ID).Error; err != nil {
			t.Fatalf("não encontrou subscription válida: %v", err)
		}
		if updatedValid.Status != string(domain.StatusActive) {
			t.Errorf("subscription válida não deve ser expirada, status obtido: '%s'", updatedValid.Status)
		}
		if updatedValid.CutsReservedInPeriod != 1 {
			t.Errorf("subscription válida: cuts_reserved não deve mudar (esperado 1, obtido %d)", updatedValid.CutsReservedInPeriod)
		}

		// ── Assert: appointments futuros limpos ──
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

		// ── Assert: appointment passado/completed intacto ──
		var apCompletedAfter models.Appointment
		if err := tx.First(&apCompletedAfter, apCompleted.ID).Error; err != nil {
			t.Fatalf("não encontrou appointment completed: %v", err)
		}
		if !apCompletedAfter.ReservedSubscriptionCut {
			t.Errorf("appointment completed não deve ter reserved_subscription_cut alterado")
		}
		if apCompletedAfter.CoverageStatus != models.CoverageStatusCovered {
			t.Errorf("appointment completed não deve ter coverage_status alterado (esperado 'covered', obtido '%s')",
				apCompletedAfter.CoverageStatus)
		}

		// ── Assert: appointment cancelled intacto ──
		var apCancelledAfter models.Appointment
		if err := tx.First(&apCancelledAfter, apCancelled.ID).Error; err != nil {
			t.Fatalf("não encontrou appointment cancelled: %v", err)
		}
		if !apCancelledAfter.ReservedSubscriptionCut {
			t.Errorf("appointment cancelled não deve ter reserved_subscription_cut alterado")
		}

		return errors.New("rollback intencional")
	})

	if outerErr != nil && outerErr.Error() != "rollback intencional" {
		t.Errorf("transação de teste falhou inesperadamente: %v", outerErr)
	}
}

// TestExpireSubscriptions_NaoExpiraSubscricaoValida confirma que uma subscription
// com current_period_end no futuro não é afetada.
func TestExpireSubscriptions_NaoExpiraSubscricaoValida(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	_ = db.Transaction(func(tx *gorm.DB) error {
		bs := seedBarbershop(t, tx)
		cl := seedClient(t, tx, bs.ID)
		plan := seedPlan(t, tx, bs.ID)
		sub := seedValidSub(t, tx, bs.ID, cl.ID, plan.ID)

		repo := NewSubscriptionGormRepository(tx)
		if _, err := repo.ExpireSubscriptions(ctx); err != nil {
			t.Errorf("ExpireSubscriptions retornou erro: %v", err)
		}

		var updatedSub models.Subscription
		if err := tx.First(&updatedSub, sub.ID).Error; err != nil {
			t.Fatalf("não encontrou subscription: %v", err)
		}
		if updatedSub.Status != string(domain.StatusActive) {
			t.Errorf("subscription válida não deve ser expirada, status obtido: '%s'", updatedSub.Status)
		}

		return errors.New("rollback intencional")
	})
}

// TestExpireSubscriptions_MultiplasSubscriptionsEmLote valida que o batch expira
// e limpa appointments de múltiplas subscriptions na mesma execução.
func TestExpireSubscriptions_MultiplasSubscriptionsEmLote(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	outerErr := db.Transaction(func(tx *gorm.DB) error {
		bs := seedBarbershop(t, tx)
		plan := seedPlan(t, tx, bs.ID)

		// Dois clientes com subscriptions expiradas.
		cl1 := seedClient(t, tx, bs.ID)
		cl2 := seedClient(t, tx, bs.ID)

		sub1 := seedExpiredSub(t, tx, bs.ID, cl1.ID, plan.ID, 1)
		sub2 := seedExpiredSub(t, tx, bs.ID, cl2.ID, plan.ID, 1)

		sub1ID := sub1.ID
		sub2ID := sub2.ID
		cl1ID := cl1.ID
		cl2ID := cl2.ID

		ap1 := seedAppointment(t, tx, bs.ID, &cl1ID, &sub1ID,
			models.AppointmentStatusScheduled, now.Add(24*time.Hour), true)
		ap2 := seedAppointment(t, tx, bs.ID, &cl2ID, &sub2ID,
			models.AppointmentStatusScheduled, now.Add(24*time.Hour), true)

		repo := NewSubscriptionGormRepository(tx)
		n, err := repo.ExpireSubscriptions(ctx)
		if err != nil {
			t.Errorf("ExpireSubscriptions retornou erro: %v", err)
			return errors.New("rollback — falha no act")
		}
		if n < 2 {
			t.Errorf("esperado pelo menos 2 subscriptions expiradas, obtido %d", n)
		}

		for _, apID := range []uint{ap1.ID, ap2.ID} {
			var ap models.Appointment
			if err := tx.First(&ap, apID).Error; err != nil {
				t.Fatalf("não encontrou appointment %d: %v", apID, err)
			}
			if ap.ReservedSubscriptionCut {
				t.Errorf("appointment %d: reserved_subscription_cut deve ser false após expiração", apID)
			}
			if ap.CoverageStatus != models.CoverageStatusNotCoveredExpired {
				t.Errorf("appointment %d: coverage_status esperado 'not_covered_expired', obtido '%s'",
					apID, ap.CoverageStatus)
			}
		}

		return errors.New("rollback intencional")
	})

	if outerErr != nil && outerErr.Error() != "rollback intencional" {
		t.Errorf("transação de teste falhou inesperadamente: %v", outerErr)
	}
}
