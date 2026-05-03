package repository

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// isPgUniqueViolation retorna true se err é uma violação de unique constraint
// com o nome informado. constraintName vazio aceita qualquer unique violation.
func isPgUniqueViolation(err error, constraintName string) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code != "23505" {
			return false
		}
		return constraintName == "" || pgErr.ConstraintName == constraintName
	}
	if constraintName != "" {
		return strings.Contains(strings.ToLower(err.Error()), constraintName)
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate key")
}
