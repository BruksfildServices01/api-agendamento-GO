package models

import "time"

// ServiceImage holds one of the up-to-3 photos for a BarbershopService.
type ServiceImage struct {
	ID                 uint               `gorm:"primaryKey"`
	BarbershopServiceID uint              `gorm:"index;not null"`
	BarbershopService  *BarbershopService `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	URL      string `gorm:"not null"`
	Position int    `gorm:"not null;default:0"` // 0, 1, 2

	CreatedAt time.Time
}
