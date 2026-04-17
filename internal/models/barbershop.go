package models

import "time"

type Barbershop struct {
	ID                uint                     `gorm:"primaryKey"`
	Name              string                   `gorm:"size:100;not null"`
	Slug              string                   `gorm:"size:100;uniqueIndex;not null"`
	Phone             string                   `gorm:"size:20"`
	Address      string `gorm:"size:255"`
	CEP          string `gorm:"size:9;column:cep"`
	StreetName   string `gorm:"size:255;column:street_name"`
	StreetNumber string `gorm:"size:20;column:street_number"`
	Complement   string `gorm:"size:100"`
	Neighborhood string `gorm:"size:100"`
	City         string `gorm:"size:100"`
	State        string `gorm:"size:2"`
	Email             string                   `gorm:"size:255"`
	MinAdvanceMinutes          int                      `gorm:"default:120"`
	ScheduleToleranceMinutes   int                      `gorm:"default:0"`
	Timezone          string                   `gorm:"size:64;not null;default:'America/Sao_Paulo'"`
	PhotoURL          *string                  `gorm:"size:512"`
	PaymentConfig     *BarbershopPaymentConfig `gorm:"constraint:OnDelete:CASCADE;"`

	// SaaS billing
	Status                string     `gorm:"size:30;not null;default:'trial'"`
	TrialEndsAt           *time.Time `gorm:"index"`
	SubscriptionExpiresAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}
