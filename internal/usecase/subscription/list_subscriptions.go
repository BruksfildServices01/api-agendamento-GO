package subscription

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type SubscriptionListItem struct {
	ID                 uint
	ClientID           uint
	Status             string
	ClientName         string
	ClientPhone        string
	PlanName           string
	CutsUsed           int
	CutsIncluded       int
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
}

type ListSubscriptions struct {
	db *gorm.DB
}

func NewListSubscriptions(db *gorm.DB) *ListSubscriptions {
	return &ListSubscriptions{db: db}
}

func (uc *ListSubscriptions) Execute(ctx context.Context, barbershopID uint) ([]SubscriptionListItem, error) {
	type row struct {
		ID                 uint      `gorm:"column:id"`
		ClientID           uint      `gorm:"column:client_id"`
		Status             string    `gorm:"column:status"`
		ClientName         string    `gorm:"column:client_name"`
		ClientPhone        string    `gorm:"column:client_phone"`
		PlanName           string    `gorm:"column:plan_name"`
		CutsUsed           int       `gorm:"column:cuts_used"`
		CutsIncluded       int       `gorm:"column:cuts_included"`
		CurrentPeriodStart time.Time `gorm:"column:current_period_start"`
		CurrentPeriodEnd   time.Time `gorm:"column:current_period_end"`
	}

	var rows []row
	now := time.Now().UTC()

	err := uc.db.WithContext(ctx).Raw(`
		SELECT
			s.id,
			s.client_id,
			s.status,
			COALESCE(c.name,  '') AS client_name,
			COALESCE(c.phone, '') AS client_phone,
			COALESCE(p.name,  '') AS plan_name,
			s.cuts_used_in_period  AS cuts_used,
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
			CutsIncluded:       r.CutsIncluded,
			CurrentPeriodStart: r.CurrentPeriodStart,
			CurrentPeriodEnd:   r.CurrentPeriodEnd,
		}
	}
	return items, nil
}
