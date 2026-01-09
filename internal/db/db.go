package db

import (
	"log"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewDB(cfg *config.Config) *gorm.DB {
	db, err := gorm.Open(postgres.Open(cfg.DBUrl), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

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
