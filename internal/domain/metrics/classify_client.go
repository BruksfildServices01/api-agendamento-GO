package metrics

const (
	trustedMinCompleted int     = 3
	atRiskMinCancelRate float64 = 0.4
)

func Classify(m *ClientMetrics) ClientCategory {
	if m.TotalAppointments == 0 {
		return CategoryNew
	}

	cancelRate := float64(m.CancelledAppointments) / float64(m.TotalAppointments)

	switch {
	case m.CompletedAppointments >= trustedMinCompleted && cancelRate == 0:
		return CategoryTrusted

	case cancelRate >= atRiskMinCancelRate:
		return CategoryAtRisk

	default:
		return CategoryRegular
	}
}
