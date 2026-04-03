package ticket

import (
	"context"
	"errors"

	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

var ErrTicketNotFound = errors.New("ticket not found")
var ErrTokenExpired = errors.New("ticket token expired or not found")

type Repository interface {
	Upsert(ctx context.Context, ticket *models.AppointmentTicket) error
	GetByToken(ctx context.Context, token string) (*models.AppointmentTicket, error)
	GetByAppointmentID(ctx context.Context, appointmentID uint) (*models.AppointmentTicket, error)
	Save(ctx context.Context, ticket *models.AppointmentTicket) error
}
