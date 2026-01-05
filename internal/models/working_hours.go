package models

import "time"

type WorkingHours struct {
	ID       uint `gorm:"primaryKey" json:"id"`
	BarberID uint `json:"barber_id"`

	Weekday int `json:"weekday"`

	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	LunchStart string `json:"lunch_start"`
	LunchEnd   string `json:"lunch_end"`
	Active     bool   `json:"active"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
