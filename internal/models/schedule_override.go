package models

import "time"

// ScheduleOverride representa uma exceção de horário para um barbeiro em uma
// data específica ou para um dia da semana em um determinado mês/ano.
//
// Escopo mutuamente exclusivo:
//   - Date != nil  → exceção para aquela data específica (ex: 25/04/2025)
//   - Weekday != nil → exceção para todos os [weekday] do [Month]/[Year] (ex: todas as terças de abril/2025)
//
// Comportamento:
//   - Closed == true          → dia completamente fechado
//   - StartTime / EndTime set → trabalha no horário alternativo
type ScheduleOverride struct {
	ID           uint `gorm:"primaryKey"`
	BarbershopID uint `gorm:"not null"`
	BarberID     uint `gorm:"not null"`

	// escopo
	Date    *time.Time `gorm:"type:date"`
	Weekday *int       `gorm:"type:smallint"`
	Month   *int       `gorm:"type:smallint"`
	Year    *int       `gorm:"type:smallint"`

	// comportamento
	Closed    bool   `gorm:"not null;default:false"`
	StartTime string `gorm:"type:varchar(5)"` // HH:MM
	EndTime   string `gorm:"type:varchar(5)"` // HH:MM

	CreatedAt time.Time
	UpdatedAt time.Time
}
