package models

import "time"

type Barbershop struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	Name              string    `gorm:"size:100;not null" json:"name"`
	Slug              string    `gorm:"size:100;uniqueIndex;not null" json:"slug"`
	Phone             string    `gorm:"size:20" json:"phone"`
	Address           string    `gorm:"size:255" json:"address"`
	MinAdvanceMinutes int       `gorm:"default:120" json:"min_advance_minutes"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
