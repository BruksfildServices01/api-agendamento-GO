package db

import (
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
)

func NewDB(cfg *config.Config) *gorm.DB {

	// ======================================================
	// CONNECT
	// ======================================================

	// QueryExecModeSimpleProtocol sends values as text instead of binary.
	// This prevents "cache lookup failed for type OID" errors that occur when
	// the database is recreated and pgx has stale enum type OIDs cached.
	pgxCfg, err := pgx.ParseConfig(cfg.DBUrl)
	if err != nil {
		log.Fatalf("failed to parse database URL: %v", err)
	}
	pgxCfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	sqlDB := stdlib.OpenDB(*pgxCfg)

	db, err := gorm.Open(
		postgres.New(postgres.Config{Conn: sqlDB}),
		&gorm.Config{
			PrepareStmt: false,
		},
	)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// ======================================================
	// CONNECTION POOL
	// ======================================================

	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	log.Println("[DB] connected successfully (schema controlled by SQL migrations)")

	return db
}
