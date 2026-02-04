package appointment

import (
	"context"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
	paymentconfig "github.com/BruksfildServices01/barber-scheduler/internal/usecase/paymentconfig"
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
	repo domain.Repository

	audit *audit.Dispatcher

	paymentPolicy *paymentconfig.ResolveBookingPaymentPolicy
}

func NewCreatePrivateAppointment(
	repo domain.Repository,
	audit *audit.Dispatcher,
	paymentPolicy *paymentconfig.ResolveBookingPaymentPolicy,
) *CreatePrivateAppointment {
	return &CreatePrivateAppointment{
		repo:          repo,
		audit:         audit,
		paymentPolicy: paymentPolicy,
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
	// 2️⃣ Política de pagamento
	// --------------------------------------------------
	policy, err := uc.paymentPolicy.Execute(ctx, in.BarbershopID)
	if err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 3️⃣ Parse de data / hora
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
	// 4️⃣ Antecedência mínima
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
	// 5️⃣ Serviço
	// --------------------------------------------------
	product, err := uc.repo.GetProduct(ctx, in.BarbershopID, in.ProductID)
	if err != nil {
		return nil, httperr.ErrBusiness("product_not_found")
	}

	end := start.Add(time.Duration(product.DurationMin) * time.Minute)

	// --------------------------------------------------
	// 6️⃣ Horário de trabalho
	// --------------------------------------------------
	ok, err := uc.repo.IsWithinWorkingHours(ctx, in.BarberID, start, end)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httperr.ErrBusiness("outside_working_hours")
	}

	// --------------------------------------------------
	// 7️⃣ Cliente
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
	// 8️⃣ Conflito de horário
	// --------------------------------------------------
	if err := uc.repo.AssertNoTimeConflict(ctx, in.BarberID, start, end); err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 9️⃣ Status inicial
	// --------------------------------------------------
	initialStatus := domain.StatusScheduled
	if policy.RequirePix {
		initialStatus = domain.StatusAwaitingPayment
	}

	ap := &models.Appointment{
		BarbershopID:    in.BarbershopID,
		BarberID:        in.BarberID,
		ClientID:        client.ID,
		BarberProductID: product.ID,
		StartTime:       start,
		EndTime:         end,
		Status:          string(initialStatus),
		Notes:           in.Notes,
	}

	if err := uc.repo.CreateAppointment(ctx, ap); err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 🔍 Auditoria
	// --------------------------------------------------
	uc.audit.Dispatch(audit.Event{
		BarbershopID: in.BarbershopID,
		UserID:       &in.BarberID,
		Action:       "appointment_created",
		Entity:       "appointment",
		EntityID:     &ap.ID,
		Metadata: map[string]any{
			"status": ap.Status,
		},
	})

	return ap, nil
}
