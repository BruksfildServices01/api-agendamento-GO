package appointment

import (
	"context"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
)

// ======================================================
// INPUT
// ======================================================

type CreatePrivateAppointmentInput struct {
	BarbershopID uint
	BarberID     uint

	ClientName  string
	ClientPhone string
	ClientEmail string

	ProductID uint

	Date  string
	Time  string
	Notes string
}

// ======================================================
// USE CASE
// ======================================================

type CreatePrivateAppointment struct {
	repo  domain.Repository
	audit *audit.Dispatcher
}

func NewCreatePrivateAppointment(
	repo domain.Repository,
	audit *audit.Dispatcher,
) *CreatePrivateAppointment {
	return &CreatePrivateAppointment{
		repo:  repo,
		audit: audit,
	}
}

// ======================================================
// EXECUTE
// ======================================================

func (uc *CreatePrivateAppointment) Execute(
	ctx context.Context,
	in CreatePrivateAppointmentInput,
) (*models.Appointment, error) {

	// --------------------------------------------------
	// 1️⃣ Barbearia
	// --------------------------------------------------
	shop, err := uc.repo.GetBarbershopByID(ctx, in.BarbershopID)
	if err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 2️⃣ Data / hora no timezone da barbearia
	// --------------------------------------------------
	start, err := time.ParseInLocation(
		"2006-01-02 15:04",
		in.Date+" "+in.Time,
		timezone.Location(shop.Timezone),
	)
	if err != nil {
		return nil, httperr.ErrBusiness("invalid_date_or_time")
	}

	// --------------------------------------------------
	// 3️⃣ Antecedência mínima
	// --------------------------------------------------
	minAdvance := shop.MinAdvanceMinutes
	if minAdvance <= 0 {
		minAdvance = 120
	}

	now := timezone.NowIn(shop.Timezone)
	if start.Before(now.Add(time.Duration(minAdvance) * time.Minute)) {
		return nil, httperr.ErrBusiness("too_soon")
	}

	// --------------------------------------------------
	// 4️⃣ Serviço
	// --------------------------------------------------
	product, err := uc.repo.GetProduct(
		ctx,
		in.BarbershopID,
		in.ProductID,
	)
	if err != nil {
		return nil, httperr.ErrBusiness("product_not_found")
	}

	end := start.Add(time.Duration(product.DurationMin) * time.Minute)

	// --------------------------------------------------
	// 5️⃣ Working hours + almoço
	// --------------------------------------------------
	ok, err := uc.repo.IsWithinWorkingHours(
		ctx,
		in.BarberID,
		start,
		end,
	)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httperr.ErrBusiness("outside_working_hours")
	}

	// --------------------------------------------------
	// 6️⃣ Cliente (get or create)
	// --------------------------------------------------
	client, err := uc.repo.GetOrCreateClient(
		ctx,
		in.BarbershopID,
		in.ClientName,
		in.ClientPhone,
		in.ClientEmail,
	)
	if err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 7️⃣ Conflito de horário
	// --------------------------------------------------
	if err := uc.repo.AssertNoTimeConflict(
		ctx,
		in.BarberID,
		start,
		end,
	); err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 8️⃣ Criação do agendamento (status centralizado)
	// --------------------------------------------------
	ap := &models.Appointment{
		BarbershopID:    in.BarbershopID,
		BarberID:        in.BarberID,
		ClientID:        client.ID,
		BarberProductID: product.ID,
		StartTime:       start,
		EndTime:         end,
		Status:          string(domain.StatusScheduled),
		Notes:           in.Notes,
	}

	if err := uc.repo.CreateAppointment(ctx, ap); err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 9️⃣ Auditoria
	// --------------------------------------------------
	uc.audit.Dispatch(audit.Event{
		BarbershopID: in.BarbershopID,
		UserID:       &in.BarberID,
		Action:       "appointment_created",
		Entity:       "appointment",
		EntityID:     &ap.ID,
	})

	return ap, nil
}
