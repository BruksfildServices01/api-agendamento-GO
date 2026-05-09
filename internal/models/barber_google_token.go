package models

import "time"

// BarberGoogleToken armazena as credenciais OAuth do Google Calendar
// de cada barbeiro individualmente.
// access_token e refresh_token são criptografados com AES-256.
type BarberGoogleToken struct {
	ID           uint      `gorm:"primaryKey"`
	UserID       uint      `gorm:"uniqueIndex;not null"`
	BarbershopID uint      `gorm:"not null;index"`
	AccessToken  string    `gorm:"type:text;not null"`
	RefreshToken string    `gorm:"type:text;not null"`
	TokenExpiry  time.Time `gorm:"not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
