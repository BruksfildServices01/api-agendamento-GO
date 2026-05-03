package appointment

import "time"

// parseHM interpreta "HH:MM" como time.Time no dia e timezone de refDay.
func parseHM(hm string, refDay time.Time, loc *time.Location) time.Time {
	t, _ := time.Parse("15:04", hm)
	return time.Date(refDay.Year(), refDay.Month(), refDay.Day(),
		t.Hour(), t.Minute(), 0, 0, loc)
}

// applyTolerance retorna o range efetivo após subtrair a tolerância das bordas.
// Se a tolerância for >= metade da duração, retorna o range original para não
// liberar slots que estariam ocupados.
func applyTolerance(start, end time.Time, toleranceMin int) (time.Time, time.Time) {
	if toleranceMin <= 0 {
		return start, end
	}
	tol := time.Duration(toleranceMin) * time.Minute
	cs := start.Add(tol)
	ce := end.Add(-tol)
	if cs.Before(ce) {
		return cs, ce
	}
	return start, end
}
