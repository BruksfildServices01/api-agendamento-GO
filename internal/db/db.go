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
	); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	return db
}
