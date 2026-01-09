package handlers

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

func writeAudit(
	db *gorm.DB,
	barbershopID uint,
	userID *uint,
	action string,
	entity string,
	entityID *uint,
	meta any,
) {

	var payload string
	if meta != nil {
		if b, err := json.Marshal(meta); err == nil {
			payload = string(b)
		}
	}

	log := models.AuditLog{
		BarbershopID: barbershopID,
		UserID:       userID,
		Action:       action,
		Entity:       entity,
		EntityID:     entityID,
		Metadata:     payload,
	}

	db.Create(&log)
}
