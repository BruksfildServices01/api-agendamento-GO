package jobs

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"
)

// PruneJob remove registros que crescem sem limite e não precisam ser retidos
// indefinidamente: audit_logs antigos, idempotency_keys expiradas e carts expirados.
//
// Retenções:
//   - audit_logs:        90 dias
//   - idempotency_keys:  30 dias  (nenhum webhook de pagamento replaya após isso)
//   - carts:             expirados há mais de 1 hora
type PruneJob struct {
	db *gorm.DB
}

func NewPruneJob(db *gorm.DB) *PruneJob {
	return &PruneJob{db: db}
}

func (j *PruneJob) Run(ctx context.Context) {
	now := time.Now().UTC()
	log.Printf("[PruneJob] started at=%s", now.Format(time.RFC3339))

	// audit_logs: mantém 90 dias
	auditCutoff := now.AddDate(0, 0, -90)
	res := j.db.WithContext(ctx).
		Exec("DELETE FROM audit_logs WHERE created_at < ?", auditCutoff)
	if res.Error != nil {
		log.Printf("[PruneJob] audit_logs error=%v", res.Error)
	} else if res.RowsAffected > 0 {
		log.Printf("[PruneJob] audit_logs deleted=%d", res.RowsAffected)
	}

	// idempotency_keys: mantém 30 dias
	idempotencyCutoff := now.AddDate(0, 0, -30)
	res = j.db.WithContext(ctx).
		Exec("DELETE FROM idempotency_keys WHERE created_at < ?", idempotencyCutoff)
	if res.Error != nil {
		log.Printf("[PruneJob] idempotency_keys error=%v", res.Error)
	} else if res.RowsAffected > 0 {
		log.Printf("[PruneJob] idempotency_keys deleted=%d", res.RowsAffected)
	}

	// carts: deleta os expirados com 1h de tolerância
	cartCutoff := now.Add(-1 * time.Hour)
	res = j.db.WithContext(ctx).
		Exec("DELETE FROM carts WHERE expires_at < ?", cartCutoff)
	if res.Error != nil {
		log.Printf("[PruneJob] carts error=%v", res.Error)
	} else if res.RowsAffected > 0 {
		log.Printf("[PruneJob] carts deleted=%d", res.RowsAffected)
	}

	log.Printf("[PruneJob] finished at=%s", time.Now().UTC().Format(time.RFC3339))
}
