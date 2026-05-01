package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	domainTicket "github.com/BruksfildServices01/barber-scheduler/internal/domain/ticket"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type TicketGormRepository struct {
	db *gorm.DB
}

func NewTicketGormRepository(db *gorm.DB) *TicketGormRepository {
	return &TicketGormRepository{db: db}
}

func (r *TicketGormRepository) Upsert(ctx context.Context, ticket *models.AppointmentTicket) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "appointment_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"token", "expires_at"}),
		}).
		Create(ticket).
		Error
}

func (r *TicketGormRepository) GetByToken(ctx context.Context, token string) (*models.AppointmentTicket, error) {
	var ticket models.AppointmentTicket
	err := r.db.WithContext(ctx).
		Where("token = ?", token).
		First(&ticket).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainTicket.ErrTicketNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (r *TicketGormRepository) GetByAppointmentID(ctx context.Context, appointmentID uint) (*models.AppointmentTicket, error) {
	var ticket models.AppointmentTicket
	err := r.db.WithContext(ctx).
		Where("appointment_id = ?", appointmentID).
		First(&ticket).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainTicket.ErrTicketNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (r *TicketGormRepository) Save(ctx context.Context, ticket *models.AppointmentTicket) error {
	return r.db.WithContext(ctx).Save(ticket).Error
}
