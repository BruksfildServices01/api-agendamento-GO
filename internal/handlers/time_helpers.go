package handlers

import (
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

const defaultTimezone = "America/Sao_Paulo"

// --------------------------------------------------
// FASE 2 — Timezone centralizado por barbearia
// --------------------------------------------------

// resolve o timezone oficial da barbearia
func locationFromShop(shop *models.Barbershop) *time.Location {
	if shop != nil && shop.Timezone != "" {
		if loc, err := time.LoadLocation(shop.Timezone); err == nil {
			return loc
		}
	}

	loc, _ := time.LoadLocation(defaultTimezone)
	return loc
}

func nowInShop(shop *models.Barbershop) time.Time {
	return time.Now().In(locationFromShop(shop))
}

func parseDateInShop(shop *models.Barbershop, dateStr string) (time.Time, error) {
	return time.ParseInLocation(
		"2006-01-02",
		dateStr,
		locationFromShop(shop),
	)
}

func parseDateTimeInShop(
	shop *models.Barbershop,
	dateStr string,
	timeStr string,
) (time.Time, error) {
	return time.ParseInLocation(
		"2006-01-02 15:04",
		dateStr+" "+timeStr,
		locationFromShop(shop),
	)
}
