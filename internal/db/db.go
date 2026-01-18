package db

import (
	"log"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewDB(cfg *config.Config) *gorm.DB {
	db, err := gorm.Open(postgres.Open(cfg.DBUrl), &gorm.Config{
		PrepareStmt: true,
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get sql.DB: %v", err)
	}

	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	if err := db.AutoMigrate(
		&models.Barbershop{},
		&models.User{},
		&models.BarberProduct{},
		&models.WorkingHours{},
		&models.Client{},
		&models.Appointment{},
		&models.AuditLog{},
	); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	db.Exec(`
        UPDATE barbershops
        SET timezone = 'America/Sao_Paulo'
        WHERE timezone IS NULL OR timezone = ''
    `)

	return db
}
