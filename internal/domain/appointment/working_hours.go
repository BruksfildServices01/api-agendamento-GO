package appointment

import (
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

// IsWithinWorkingHours valida se um horário está dentro do expediente
// incluindo pausa de almoço (regra de domínio)
func IsWithinWorkingHours(
	db *gorm.DB,
	shop *models.Barbershop,
	barberID uint,
	start time.Time,
	end time.Time,
) (bool, error) {

	weekday := int(start.Weekday())
	loc := start.Location()

	var wh models.WorkingHours
	if err := db.
		Where("barber_id = ? AND weekday = ?", barberID, weekday).
		First(&wh).Error; err != nil {
		return false, nil
	}

	if !wh.Active || wh.StartTime == "" || wh.EndTime == "" {
		return false, nil
	}

	parseHM := func(hm string) time.Time {
		t, _ := time.Parse("15:04", hm)
		return time.Date(
			start.Year(), start.Month(), start.Day(),
			t.Hour(), t.Minute(), 0, 0,
			loc,
		)
	}

	workStart := parseHM(wh.StartTime)
	workEnd := parseHM(wh.EndTime)

	if start.Before(workStart) || end.After(workEnd) {
		return false, nil
	}

	if wh.LunchStart != "" && wh.LunchEnd != "" {
		lunchStart := parseHM(wh.LunchStart)
		lunchEnd := parseHM(wh.LunchEnd)

		if start.Before(lunchEnd) && end.After(lunchStart) {
			return false, nil
		}
	}

	return true, nil
}
