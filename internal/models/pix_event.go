package models

import "time"

type PixEvent struct {
	ID        uint      `gorm:"primaryKey"`
	TxID      string    `gorm:"column:tx_id;size:100;not null"`
	EventType string    `gorm:"column:event_type;size:50;not null"`
	CreatedAt time.Time `gorm:"column:created_at"`
}
