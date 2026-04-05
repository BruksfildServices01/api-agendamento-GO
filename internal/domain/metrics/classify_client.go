package metrics

import "time"

const (
	// Trusted: mínimo de cortes concluídos sem falha nenhuma.
	trustedMinCompleted int = 5

	// AtRisk por no-show: limiar menor porque no-show é o pior sinal.
	atRiskNoShowRate float64 = 0.20

	// AtRisk por cancelamento (inclui tardios).
	atRiskCancelRate float64 = 0.40

	// Inatividade: cliente sem visita há mais de 90 dias com histórico → at_risk.
	atRiskInactiveDays = 90

	// Trusted exige visita recente (30 dias) para ser mantido ativo.
	trustedRecentDays = 30
)

// Classify derives the behavioral category for a client from their metrics.
// Rules (priority order):
//  1. No history → new
//  2. No-show rate ≥ 20% OR cancel rate ≥ 40% (with late penalties) → at_risk
//  3. Inactive > 90 days (with prior history) → at_risk
//  4. ≥ 5 completions, zero no-shows, zero cancellations, active ≤ 30 days → trusted
//  5. Default → regular
func Classify(m *ClientMetrics) ClientCategory {
	if m.TotalAppointments == 0 {
		return CategoryNew
	}

	now := time.Now().UTC()

	// Effective cancel count: cancellations + late cancellations count double.
	effectiveCancels := m.CancelledAppointments + m.LateCancelledAppointments
	cancelRate := float64(effectiveCancels) / float64(m.TotalAppointments)

	noShowRate := float64(m.NoShowAppointments) / float64(m.TotalAppointments)

	// 1. No-show is the worst signal — checked first.
	if noShowRate >= atRiskNoShowRate {
		return CategoryAtRisk
	}

	// 2. High cancel rate → at_risk.
	if cancelRate >= atRiskCancelRate {
		return CategoryAtRisk
	}

	// 3. Inactive for 90+ days with at least 2 completed appointments → at_risk.
	if m.CompletedAppointments >= 2 && m.LastCompletedAt != nil {
		if now.Sub(*m.LastCompletedAt) > atRiskInactiveDays*24*time.Hour {
			return CategoryAtRisk
		}
	}

	// 4. Trusted: 5+ completions, zero no-shows, zero cancellations, active in last 30 days.
	if m.CompletedAppointments >= trustedMinCompleted &&
		m.NoShowAppointments == 0 &&
		m.CancelledAppointments == 0 &&
		m.LateCancelledAppointments == 0 &&
		m.LastCompletedAt != nil &&
		now.Sub(*m.LastCompletedAt) <= trustedRecentDays*24*time.Hour {
		return CategoryTrusted
	}

	return CategoryRegular
}
