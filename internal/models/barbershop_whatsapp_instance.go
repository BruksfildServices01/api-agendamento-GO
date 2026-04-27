package models

import (
	"fmt"
	"time"
)

// BarbershopWhatsAppInstance representa uma conexão WhatsApp via Evolution API.
// barber_id NULL = instância da barbearia inteira (plano atual).
// barber_id preenchido = instância individual (plano multi-barbeiro futuro).
type BarbershopWhatsAppInstance struct {
	ID           uint   `gorm:"primaryKey"`
	BarbershopID uint   `gorm:"not null;index"`
	BarberID     *uint  `gorm:"index"`
	InstanceName string `gorm:"size:100;not null;uniqueIndex"`
	Phone        string `gorm:"size:20"`
	// Status: "disconnected" | "connecting" | "connected"
	Status    string `gorm:"size:20;not null;default:disconnected"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (BarbershopWhatsAppInstance) TableName() string {
	return "barbershop_whatsapp_instances"
}

// InstanceNameFor gera o nome da instância de forma consistente.
// Hoje: "bs{id}" — futuro multi-barbeiro: "bs{id}_b{barberID}".
func InstanceNameFor(barbershopID uint, barberID *uint) string {
	if barberID != nil {
		return fmt.Sprintf("bs%d_b%d", barbershopID, *barberID)
	}
	return fmt.Sprintf("bs%d", barbershopID)
}
