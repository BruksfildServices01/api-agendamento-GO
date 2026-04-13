package ticket

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	domainTicket "github.com/BruksfildServices01/barber-scheduler/internal/domain/ticket"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type GenerateTicketInput struct {
	AppointmentID uint
	BarbershopID  uint
	StartTime     time.Time
}

type GenerateTicket struct {
	repo domainTicket.Repository
}

func NewGenerateTicket(repo domainTicket.Repository) *GenerateTicket {
	return &GenerateTicket{repo: repo}
}

func (uc *GenerateTicket) Execute(ctx context.Context, input GenerateTicketInput) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)

	ticket := &models.AppointmentTicket{
		AppointmentID: input.AppointmentID,
		BarbershopID:  input.BarbershopID,
		Token:         token,
		// Ticket stays accessible for 30 days after the appointment so the
		// client can still view details or contact the barbershop.
		ExpiresAt: input.StartTime.Add(30 * 24 * time.Hour),
	}

	if err := uc.repo.Upsert(ctx, ticket); err != nil {
		return "", err
	}

	return token, nil
}
