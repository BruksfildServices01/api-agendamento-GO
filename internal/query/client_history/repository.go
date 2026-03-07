package clienthistory

import "gorm.io/gorm"

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetClientHistory(
	barbershopID int64,
	clientID int64,
) (*ClientHistoryDTO, error) {

	var dto ClientHistoryDTO

	err := r.db.Raw(`
		SELECT
			c.id AS client_id,

			COUNT(a.id) AS appointments_total,

			COUNT(*) FILTER (WHERE a.status = 'completed') AS attended,
			COUNT(*) FILTER (WHERE a.status = 'no_show')   AS missed,
			COUNT(*) FILTER (WHERE a.status = 'cancelled') AS cancelled,

			MAX(a.start_time) AS last_appointment_at

		FROM clients c
		LEFT JOIN appointments a
			ON a.client_id = c.id
			AND a.barbershop_id = ?

		WHERE c.id = ?
			AND c.barbershop_id = ?

		GROUP BY c.id;
	`, barbershopID, clientID, barbershopID).
		Scan(&dto).Error

	if err != nil {
		return nil, err
	}

	return &dto, nil
}
