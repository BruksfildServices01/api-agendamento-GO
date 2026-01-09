package audit

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type Logger struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Logger {
	return &Logger{db: db}
}

func (l *Logger) Log(
	barbershopID uint,
	userID *uint,
	action string,
	entity string,
	entityID *uint,
	metadata any,
) error {

	var metaJSON string
	if metadata != nil {
		if b, err := json.Marshal(metadata); err == nil {
			metaJSON = string(b)
		}
	}

	log := models.AuditLog{
		BarbershopID: barbershopID,
		UserID:       userID,
		Action:       action,
		Entity:       entity,
		EntityID:     entityID,
		Metadata:     metaJSON,
	}

	return l.db.Create(&log).Error
}
