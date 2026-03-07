package clienthistory

import "time"

// ClientHistoryDTO representa uma visão consolidada e somente leitura
// do comportamento do cliente dentro da barbearia.
type ClientHistoryDTO struct {
	ClientID int64 `json:"client_id"`

	// Classificação atual do cliente
	Category string `json:"category"`

	// Contadores principais
	AppointmentsTotal int `json:"appointments_total"`
	Attended          int `json:"attended"`
	Missed            int `json:"missed"`
	Cancelled         int `json:"cancelled"`
	Rescheduled       int `json:"rescheduled"`

	// Datas relevantes
	LastAppointmentAt *time.Time `json:"last_appointment_at,omitempty"`

	// Métricas derivadas (pré-calculadas no backend)
	AttendanceRate float64 `json:"attendance_rate"`

	// Sinais rápidos para decisão operacional
	Flags []ClientHistoryFlag `json:"flags"`
}
