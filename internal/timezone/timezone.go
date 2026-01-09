package timezone

import "time"

const DefaultTimezone = "America/Sao_Paulo"

// --------------------------------------------------
// Validação forte de timezone IANA
// --------------------------------------------------

func IsValid(tz string) bool {
	if tz == "" {
		return false
	}
	_, err := time.LoadLocation(tz)
	return err == nil
}

// --------------------------------------------------
// Resolve timezone com fallback seguro
// --------------------------------------------------

func Location(tz string) *time.Location {
	if IsValid(tz) {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}

	loc, _ := time.LoadLocation(DefaultTimezone)
	return loc
}

// --------------------------------------------------
// Helpers de tempo
// --------------------------------------------------

func Now() time.Time {
	return time.Now().In(Location(DefaultTimezone))
}

func NowIn(tz string) time.Time {
	return time.Now().In(Location(tz))
}
