package appointment

import (
	"context"
	"time"

	domainAppointment "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

type CreateInternalAppointment struct {
	appointmentRepo domainAppointment.Repository
}

func NewCreateInternalAppointment(
	appointmentRepo domainAppointment.Repository,
) *CreateInternalAppointment {
	return &CreateInternalAppointment{
		appointmentRepo: appointmentRepo,
	}
}

//
// ======================================================
// INPUT
// ======================================================
//

type CreateInternalAppointmentInput struct {
	BarbershopID uint
	BarberID     uint

	ClientName  string
	ClientPhone string
	ClientEmail string

	BarberProductID uint

	StartTime time.Time
	EndTime   time.Time

	PaymentIntent string // "paid" | "pay_later"
	Notes         string
}

//
// ======================================================
// EXECUTE
// ======================================================
//

func (uc *CreateInternalAppointment) Execute(
	ctx context.Context,
	input CreateInternalAppointmentInput,
) (*models.Appointment, error) {

	// --------------------------------------------------
	// 1️⃣ Validações mínimas
	// --------------------------------------------------

	if input.BarbershopID == 0 || input.BarberID == 0 {
		return nil, httperr.ErrBusiness("invalid_context")
	}

	if !input.StartTime.Before(input.EndTime) {
		return nil, httperr.ErrBusiness("invalid_time_range")
	}

	// --------------------------------------------------
	// 2️⃣ Validar PaymentIntent
	// --------------------------------------------------

	intent := models.PaymentIntentType(input.PaymentIntent)

	if intent != models.PaymentIntentPaid &&
		intent != models.PaymentIntentPayLater {
		return nil, httperr.ErrBusiness("invalid_payment_intent")
	}

	// --------------------------------------------------
	// 3️⃣ Cliente (get or create)
	// --------------------------------------------------

	client, err := uc.appointmentRepo.GetOrCreateClient(
		ctx,
		input.BarbershopID,
		input.ClientName,
		input.ClientPhone,
		input.ClientEmail,
	)
	if err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 4️⃣ Conflito de horário
	// --------------------------------------------------

	if err := uc.appointmentRepo.AssertNoTimeConflict(
		ctx,
		input.BarbershopID,
		input.BarberID,
		input.StartTime,
		input.EndTime,
	); err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 5️⃣ Criar Appointment
	// --------------------------------------------------

	barbershopID := input.BarbershopID
	barberID := input.BarberID
	clientID := client.ID
	productID := input.BarberProductID

	appointment := &models.Appointment{
		BarbershopID:    &barbershopID,
		BarberID:        &barberID,
		ClientID:        &clientID,
		BarberProductID: &productID,

		StartTime: input.StartTime,
		EndTime:   input.EndTime,

		Status:        models.AppointmentStatus(domainAppointment.StatusScheduled),
		CreatedBy:     models.CreatedByBarber,
		PaymentIntent: intent,

		Notes: input.Notes,
	}

	if err := uc.appointmentRepo.CreateAppointment(ctx, appointment); err != nil {
		return nil, err
	}

	return appointment, nil
}
