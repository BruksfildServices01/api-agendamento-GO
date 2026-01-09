package models

import "time"

type AuditLog struct {
	ID uint `gorm:"primaryKey" json:"id"`

	BarbershopID uint   `json:"barbershop_id"`
	UserID       *uint  `json:"user_id"`
	Action       string `gorm:"size:50;not null" json:"action"`

	Entity   string `gorm:"size:50" json:"entity"`
	EntityID *uint  `json:"entity_id"`
	Metadata string `gorm:"type:text" json:"metadata"`

	CreatedAt time.Time `json:"created_at"`
}
