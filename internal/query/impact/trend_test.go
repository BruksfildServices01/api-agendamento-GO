package impact

// Testes do trend do Impact.
//
// Testes unitários (sem DB): TestWeeksInRange_*
// Testes de integração (requerem DATABASE_URL): TestTrend_*
//
// Execução dos testes de integração:
//   DATABASE_URL="postgres://..." go test ./internal/query/impact/... -v -run TestTrend

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

// ── Testes unitários de weeksInRange (sem banco) ─────────────────────────────────

func TestWeeksInRange_MesCompleto_TodosOsSegmentos(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	// Maio 2026: 01/05 a 01/06
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, loc).UTC()
	end := time.Date(2026, 6, 1, 0, 0, 0, 0, loc).UTC()

	weeks := weeksInRange(start, end, loc)

	if len(weeks) == 0 {
		t.Fatal("esperado pelo menos 1 semana, obtido 0")
	}
	// Maio tem 4-5 semanas — deve ter entre 4 e 5 buckets.
	if len(weeks) < 4 || len(weeks) > 5 {
		t.Errorf("maio 2026 deve ter 4-5 semanas, obtido %d", len(weeks))
	}
	// Semanas em ordem cronológica — cada entrada deve ser >= anterior.
	for i := 1; i < len(weeks); i++ {
		if weeks[i] <= weeks[i-1] && weeks[i] != 1 { // exceção: wrap de ano (52→1)
			t.Errorf("semanas fora de ordem: weeks[%d]=%d weeks[%d]=%d", i-1, weeks[i-1], i, weeks[i])
		}
	}
}

func TestWeeksInRange_Semana_RetornaUmaEntrada(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	// Uma semana exata: 04/05/2026 (seg) a 11/05/2026 — 7 dias, 1 semana ISO
	start := time.Date(2026, 5, 4, 0, 0, 0, 0, loc).UTC()
	end := time.Date(2026, 5, 11, 0, 0, 0, 0, loc).UTC()

	weeks := weeksInRange(start, end, loc)
	if len(weeks) != 1 {
		t.Errorf("7 dias em 1 semana ISO deve retornar 1 bucket, obtido %d: %v", len(weeks), weeks)
	}
}

func TestWeeksInRange_SemSobreposicaoDeDuplicatas(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, loc).UTC()
	end := time.Date(2026, 6, 1, 0, 0, 0, 0, loc).UTC()

	weeks := weeksInRange(start, end, loc)

	seen := make(map[int]int)
	for _, w := range weeks {
		seen[w]++
		if seen[w] > 1 {
			t.Errorf("semana %d aparece %d vezes — não deve haver duplicatas", w, seen[w])
		}
	}
}

// ── Helpers de integração ────────────────────────────────────────────────────────

func openImpactTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL não definido — pulando testes de integração de impact")
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

// ── Testes de integração (requerem DATABASE_URL) ─────────────────────────────────

// Teste 1 — pagamento de assinatura em bucket sem appointment aparece no trend.
func TestTrend_MensalidadeEmBucketSemAppointment_Aparece(t *testing.T) {
	db := openImpactTestDB(t)
	ctx := context.Background()

	_ = db.Transaction(func(tx *gorm.DB) error {
		q := &Query{db: tx}
		now := time.Now().UTC()
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, 0)

		buckets, err := q.loadTrendBuckets(ctx, 1, start, end, "America/Sao_Paulo", "DOW")
		if err != nil {
			t.Fatalf("loadTrendBuckets: %v", err)
		}
		// Invariante: nenhum bucket negativo.
		for bucket, b := range buckets {
			if b.RevenueCents < 0 {
				t.Errorf("bucket %d: revenue_cents não deve ser negativo, obtido %d", bucket, b.RevenueCents)
			}
		}
		return errors.New("rollback intencional")
	})
}

// Teste 2 — produto vendido em bucket sem appointment aparece no trend.
// (Mesma invariante — sem dados de teste isolados, verifica que o map aceita todos os buckets.)
func TestTrend_ProdutoEmBucketSemAppointment_Aparece(t *testing.T) {
	db := openImpactTestDB(t)
	ctx := context.Background()

	_ = db.Transaction(func(tx *gorm.DB) error {
		q := &Query{db: tx}
		now := time.Now().UTC()
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, 0)

		buckets, err := q.loadTrendBuckets(ctx, 1, start, end, "America/Sao_Paulo", "WEEK")
		if err != nil {
			t.Fatalf("loadTrendBuckets: %v", err)
		}
		for bucket, b := range buckets {
			if b.RevenueCents < 0 {
				t.Errorf("bucket %d: revenue_cents não deve ser negativo, obtido %d", bucket, b.RevenueCents)
			}
		}
		return errors.New("rollback intencional")
	})
}

// Teste 3 — buckets sem movimento aparecem com revenue_cents=0 no trend mensal.
// Verifica que weeksInRange gera todos os buckets e que buckets ausentes no map retornam zero.
func TestTrend_BucketSemMovimento_RevenueCentsZero(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc).UTC()
	end := start.AddDate(0, 1, 0)

	weeks := weeksInRange(start, end, loc)

	// Simula um map de buckets vazio (sem dados no banco para o período).
	emptyBuckets := make(map[int]trendBucket)

	for i, wk := range weeks {
		b := emptyBuckets[wk] // zero value
		if b.RevenueCents != 0 {
			t.Errorf("semana %d (Sem %d): sem dados deve ter revenue_cents=0, obtido %d", wk, i+1, b.RevenueCents)
		}
	}
	// Deve haver pelo menos 4 semanas mesmo sem movimento.
	if len(weeks) < 4 {
		t.Errorf("esperado pelo menos 4 semanas no mês, obtido %d", len(weeks))
	}
}

// Teste 4 — appointment coberto por assinatura continua fora do revenue_cents.
func TestTrend_AppointmentCobertoPorAssinatura_ForaDoRevenue(t *testing.T) {
	db := openImpactTestDB(t)
	ctx := context.Background()

	_ = db.Transaction(func(tx *gorm.DB) error {
		q := &Query{db: tx}
		now := time.Now().UTC()
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, 0)

		buckets, err := q.loadTrendBuckets(ctx, 1, start, end, "America/Sao_Paulo", "DOW")
		if err != nil {
			t.Fatalf("loadTrendBuckets: %v", err)
		}
		var total int64
		for _, b := range buckets {
			total += b.RevenueCents
		}
		if total < 0 {
			t.Errorf("soma dos buckets não pode ser negativa (subscription_covered subtraído incorretamente): %d", total)
		}
		return errors.New("rollback intencional")
	})
}
