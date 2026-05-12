package subscription

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type SubscriptionListItem struct {
	ID                 uint      `json:"id"`
	ClientID           uint      `json:"client_id"`
	Status             string    `json:"status"`
	ClientName         string    `json:"client_name"`
	ClientPhone        string    `json:"client_phone"`
	PlanName           string    `json:"plan_name"`
	CutsUsed           int       `json:"cuts_used"`
	CutsReserved       int       `json:"cuts_reserved"`
	CutsIncluded       int       `json:"cuts_included"`
	CurrentPeriodStart time.Time `json:"current_period_start"`
	CurrentPeriodEnd   time.Time `json:"current_period_end"`
}

type Query struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Query {
	return &Query{db: db}
}

func (q *Query) ListActive(ctx context.Context, barbershopID uint) ([]SubscriptionListItem, error) {
	type row struct {
		ID                 uint      `gorm:"column:id"`
		ClientID           uint      `gorm:"column:client_id"`
		Status             string    `gorm:"column:status"`
		ClientName         string    `gorm:"column:client_name"`
		ClientPhone        string    `gorm:"column:client_phone"`
		PlanName           string    `gorm:"column:plan_name"`
		CutsUsed           int       `gorm:"column:cuts_used"`
		CutsReserved       int       `gorm:"column:cuts_reserved"`
		CutsIncluded       int       `gorm:"column:cuts_included"`
		CurrentPeriodStart time.Time `gorm:"column:current_period_start"`
		CurrentPeriodEnd   time.Time `gorm:"column:current_period_end"`
	}

	var rows []row
	now := time.Now().UTC()

	err := q.db.WithContext(ctx).Raw(`
		SELECT
			s.id,
			s.client_id,
			s.status,
			COALESCE(c.name,  '') AS client_name,
			COALESCE(c.phone, '') AS client_phone,
			COALESCE(p.name,  '') AS plan_name,
			s.cuts_used_in_period     AS cuts_used,
			s.cuts_reserved_in_period AS cuts_reserved,
			p.cuts_included,
			s.current_period_start,
			s.current_period_end
		FROM subscriptions s
		JOIN clients c ON c.id = s.client_id
		JOIN plans   p ON p.id = s.plan_id
		WHERE s.barbershop_id = ?
		  AND s.status = 'active'
		  AND s.current_period_start <= ?
		  AND s.current_period_end   >  ?
		ORDER BY s.current_period_end DESC
	`, barbershopID, now, now).Scan(&rows).Error

	if err != nil {
		return nil, err
	}

	items := make([]SubscriptionListItem, len(rows))
	for i, r := range rows {
		items[i] = SubscriptionListItem{
			ID:                 r.ID,
			ClientID:           r.ClientID,
			Status:             r.Status,
			ClientName:         r.ClientName,
			ClientPhone:        r.ClientPhone,
			PlanName:           r.PlanName,
			CutsUsed:           r.CutsUsed,
			CutsReserved:       r.CutsReserved,
			CutsIncluded:       r.CutsIncluded,
			CurrentPeriodStart: r.CurrentPeriodStart,
			CurrentPeriodEnd:   r.CurrentPeriodEnd,
		}
	}
	return items, nil
}
