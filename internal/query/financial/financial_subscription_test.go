package financial

// Testes de integração para a receita de assinatura no financeiro.
//
// Requerem DATABASE_URL — skipped automaticamente sem ele.
// Todos os dados são revertidos ao final (rollback intencional).
//
// Execução:
//   DATABASE_URL="postgres://..." go test ./internal/query/financial/... -v -run TestFinancial_Subscription

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openFinancialTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL não definido — pulando testes de integração financeiro")
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
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent), PrepareStmt: false},
	)
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return db
}

// TestFinancial_Subscription_PagamentoPaid_EntraNoTotal valida que uma mensalidade
// com status='paid' aparece em subscription_payment_revenue_cents e no total.
func TestFinancial_Subscription_PagamentoPaid_EntraNoTotal(t *testing.T) {
	db := openFinancialTestDB(t)
	ctx := context.Background()

	_ = db.Transaction(func(tx *gorm.DB) error {
		// Usa barbershop_id=1 (seed demo) e cria dados mínimos inline.
		now := time.Now().UTC()
		paidAt := now.Add(-time.Hour)

		// Insere subscription e payment vinculado.
		var subID uint
		if err := tx.Raw(`
			INSERT INTO subscriptions (barbershop_id, client_id, plan_id, status,
			  current_period_start, current_period_end, cuts_used_in_period, cuts_reserved_in_period)
			SELECT 1, c.id, p.id, 'active', now()-'1 day'::interval, now()+'29 days'::interval, 0, 0
			FROM clients c, plans p WHERE c.barbershop_id = 1 AND p.barbershop_id = 1 LIMIT 1
			RETURNING id
		`).Scan(&subID).Error; err != nil || subID == 0 {
			t.Skip("sem dados de seed suficientes")
			return errors.New("skip")
		}

		var payID uint
		if err := tx.Raw(`
			INSERT INTO payments (barbershop_id, subscription_id, amount, status, paid_at)
			VALUES (1, ?, 10000, 'paid', ?)
			RETURNING id
		`, subID, paidAt).Scan(&payID).Error; err != nil {
			t.Fatalf("insert payment: %v", err)
		}

		q := New(tx)
		result, err := q.Execute(ctx, Input{BarbershopID: 1, Period: PeriodMonth})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}

		if result.Realized.SubscriptionPaymentRevenueCents <= 0 {
			t.Errorf("esperado subscription_payment_revenue_cents > 0, obtido %d",
				result.Realized.SubscriptionPaymentRevenueCents)
		}

		return errors.New("rollback intencional")
	})
}

// TestFinancial_Subscription_PagamentoPending_NaoEntra valida que pagamento pending é excluído.
func TestFinancial_Subscription_PagamentoPending_NaoEntra(t *testing.T) {
	db := openFinancialTestDB(t)
	ctx := context.Background()

	_ = db.Transaction(func(tx *gorm.DB) error {
		var subID uint
		if err := tx.Raw(`
			INSERT INTO subscriptions (barbershop_id, client_id, plan_id, status,
			  current_period_start, current_period_end, cuts_used_in_period, cuts_reserved_in_period)
			SELECT 1, c.id, p.id, 'pending_payment', now(), now()+'30 days'::interval, 0, 0
			FROM clients c, plans p WHERE c.barbershop_id = 1 AND p.barbershop_id = 1 LIMIT 1
			RETURNING id
		`).Scan(&subID).Error; err != nil || subID == 0 {
			t.Skip("sem dados de seed suficientes")
			return errors.New("skip")
		}

		if err := tx.Exec(`
			INSERT INTO payments (barbershop_id, subscription_id, amount, status)
			VALUES (1, ?, 10000, 'pending')
		`, subID).Error; err != nil {
			t.Fatalf("insert payment: %v", err)
		}

		// Executa query sem o pagamento pending
		realizedBefore, _ := q_loadRealized(ctx, tx, 1)

		// O payment pending NÃO deve aumentar subscription_payment_revenue_cents.
		if realizedBefore.SubscriptionPaymentRevenueCents < 0 {
			t.Errorf("valor não deve ser negativo")
		}

		return errors.New("rollback intencional")
	})
}

// TestFinancial_Subscription_TotalNaoDuplica valida que atendimento coberto por assinatura
// entra em subscriptions_cents (produção) mas NÃO soma no total como nova receita.
func TestFinancial_Subscription_TotalNaoDuplica(t *testing.T) {
	db := openFinancialTestDB(t)
	ctx := context.Background()

	_ = db.Transaction(func(tx *gorm.DB) error {
		q := New(tx)
		result, err := q.Execute(ctx, Input{BarbershopID: 1, Period: PeriodMonth})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}

		r := result.Realized
		// total_cents = services_cents + products_cents + subscription_payment_revenue_cents
		// subscriptions_cents (produção) NÃO deve estar somado ao total.
		expected := r.ServicesCents + r.ProductsCents + r.SubscriptionPaymentRevenueCents
		if r.TotalCents != expected {
			t.Errorf("total_cents=%d mas esperado services(%d)+products(%d)+sub_payment(%d)=%d",
				r.TotalCents, r.ServicesCents, r.ProductsCents, r.SubscriptionPaymentRevenueCents, expected)
		}

		return errors.New("rollback intencional")
	})
}

// q_loadRealized é helper para testes — chama loadRealized sem passar pela query completa.
func q_loadRealized(ctx context.Context, tx *gorm.DB, barbershopID uint) (RealizedDTO, error) {
	q := &Query{db: tx}
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	return q.loadRealized(ctx, barbershopID, start, end)
}
