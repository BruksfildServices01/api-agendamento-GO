package jobs

import (
	"context"
	"fmt"
	"os"
	"time"

	"gorm.io/gorm"
)

// JobLocker faz "leader election" por job name (TTL-based).
type JobLocker interface {
	// TryLock tenta pegar o lock. Retorna:
	// - true: lock adquirido (pode executar o job)
	// - false: outro nó está com lock válido (não executa)
	TryLock(ctx context.Context, jobName string, ttl time.Duration) (bool, error)

	// Unlock é opcional (best-effort). Mesmo sem chamar, o TTL expira.
	Unlock(ctx context.Context, jobName string) error
}

type PostgresJobLocker struct {
	db    *gorm.DB
	owner string
}

func NewPostgresJobLocker(db *gorm.DB, owner string) *PostgresJobLocker {
	if owner == "" {
		owner = defaultOwner()
	}
	return &PostgresJobLocker{
		db:    db,
		owner: owner,
	}
}

func defaultOwner() string {
	host, _ := os.Hostname()
	return fmt.Sprintf("%s:%d", host, os.Getpid())
}

func (l *PostgresJobLocker) TryLock(
	ctx context.Context,
	jobName string,
	ttl time.Duration,
) (bool, error) {

	if jobName == "" {
		return false, fmt.Errorf("jobName is required")
	}
	if ttl <= 0 {
		ttl = 60 * time.Second
	}

	lockedUntil := time.Now().UTC().Add(ttl)

	// Regra:
	// - INSERT se não existe
	// - UPDATE se existe mas está expirado (locked_until <= now())
	// - se ainda está válido, não atualiza => RowsAffected=0
	res := l.db.WithContext(ctx).Exec(`
		INSERT INTO job_locks (job_name, locked_until, locked_by, updated_at)
		VALUES (?, ?, ?, now())
		ON CONFLICT (job_name) DO UPDATE
		SET locked_until = EXCLUDED.locked_until,
		    locked_by    = EXCLUDED.locked_by,
		    updated_at   = now()
		WHERE job_locks.locked_until <= now()
	`, jobName, lockedUntil, l.owner)

	if res.Error != nil {
		return false, res.Error
	}

	return res.RowsAffected == 1, nil
}

func (l *PostgresJobLocker) Unlock(ctx context.Context, jobName string) error {
	if jobName == "" {
		return nil
	}

	// Best-effort: só libera se eu for o owner atual.
	// (Se não bater, TTL resolve sozinho.)
	res := l.db.WithContext(ctx).Exec(`
		UPDATE job_locks
		SET locked_until = now() - interval '1 second',
		    updated_at = now()
		WHERE job_name = ? AND locked_by = ?
	`, jobName, l.owner)

	return res.Error
}
