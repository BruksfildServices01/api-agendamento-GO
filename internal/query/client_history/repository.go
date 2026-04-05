package clienthistory

import (
	"context"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetClientHistory(
	ctx context.Context,
	barbershopID int64,
	clientID int64,
) (*ClientHistoryDTO, error) {

	var dto ClientHistoryDTO

	err := r.db.WithContext(ctx).Raw(`
		SELECT
			c.id AS client_id,

			COALESCE(cm.total_appointments, 0)       AS appointments_total,
			COALESCE(cm.completed_appointments, 0)   AS attended,
			COALESCE(cm.no_show_appointments, 0)     AS missed,
			COALESCE(cm.cancelled_appointments, 0)   AS cancelled,
			COALESCE(cm.rescheduled_appointments, 0) AS rescheduled,

			COALESCE(cm.last_appointment_at, MAX(a.start_time)) AS last_appointment_at

		FROM clients c
		LEFT JOIN client_metrics cm
			ON cm.client_id = c.id
			AND cm.barbershop_id = c.barbershop_id
		LEFT JOIN appointments a
			ON a.client_id = c.id
			AND a.barbershop_id = c.barbershop_id

		WHERE c.id = ?
			AND c.barbershop_id = ?

		GROUP BY
			c.id,
			cm.total_appointments,
			cm.completed_appointments,
			cm.no_show_appointments,
			cm.cancelled_appointments,
			cm.rescheduled_appointments,
			cm.last_appointment_at;
	`, clientID, barbershopID).
		Scan(&dto).Error

	if err != nil {
		return nil, err
	}

	return &dto, nil
}
