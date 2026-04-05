package appointment

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

const adjustmentWindowDays = 7

var ErrAdjustmentWindowExpired = errors.New("adjustment window of 7 days has expired")
var ErrClosureNotFound = errors.New("appointment closure not found")
var ErrNoAdjustmentFields = errors.New("at least one field must be adjusted")

type AdjustClosureInput struct {
	BarbershopID  uint
	BarberID      uint
	AppointmentID uint

	// Delta fields — nil means "keep original value".
	DeltaFinalAmountCents *int64
	DeltaPaymentMethod    *string
	DeltaOperationalNote  *string

	Reason string
}

type AdjustClosure struct {
	db    *gorm.DB
	audit *audit.Dispatcher
}

func NewAdjustClosure(db *gorm.DB, audit *audit.Dispatcher) *AdjustClosure {
	return &AdjustClosure{db: db, audit: audit}
}

func (uc *AdjustClosure) Execute(
	ctx context.Context,
	input AdjustClosureInput,
) (*models.ClosureAdjustment, error) {

	if input.DeltaFinalAmountCents == nil &&
		input.DeltaPaymentMethod == nil &&
		input.DeltaOperationalNote == nil {
		return nil, ErrNoAdjustmentFields
	}

	if input.DeltaFinalAmountCents != nil && *input.DeltaFinalAmountCents < 0 {
		return nil, httperr.ErrBusiness("invalid_final_amount")
	}

	var adjustment *models.ClosureAdjustment

	err := uc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Load the closure for this appointment
		var closure models.AppointmentClosure
		if err := tx.
			Where("appointment_id = ? AND barbershop_id = ?", input.AppointmentID, input.BarbershopID).
			First(&closure).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrClosureNotFound
			}
			return err
		}

		// Enforce the 7-day adjustment window
		if time.Now().UTC().After(closure.CreatedAt.Add(adjustmentWindowDays * 24 * time.Hour)) {
			return ErrAdjustmentWindowExpired
		}

		now := time.Now().UTC()
		barberID := input.BarberID

		adjustment = &models.ClosureAdjustment{
			ClosureID:             closure.ID,
			BarbershopID:          input.BarbershopID,
			BarberID:              &barberID,
			DeltaFinalAmountCents: input.DeltaFinalAmountCents,
			DeltaPaymentMethod:    input.DeltaPaymentMethod,
			DeltaOperationalNote:  input.DeltaOperationalNote,
			Reason:                input.Reason,
			AdjustedAt:            now,
		}

		return tx.Create(adjustment).Error
	})

	if err != nil {
		return nil, err
	}

	// Audit — differentiates closure correction from original closure
	metadata := map[string]any{
		"appointment_id": input.AppointmentID,
		"closure_id":     adjustment.ClosureID,
		"reason":         input.Reason,
	}
	if input.DeltaFinalAmountCents != nil {
		metadata["delta_final_amount_cents"] = *input.DeltaFinalAmountCents
	}
	if input.DeltaPaymentMethod != nil {
		metadata["delta_payment_method"] = *input.DeltaPaymentMethod
	}
	if input.DeltaOperationalNote != nil {
		metadata["delta_operational_note"] = *input.DeltaOperationalNote
	}

	barberID := input.BarberID
	uc.audit.Dispatch(audit.Event{
		BarbershopID: input.BarbershopID,
		UserID:       &barberID,
		Action:       "closure_adjusted",
		Entity:       "appointment_closure",
		EntityID:     &adjustment.ClosureID,
		Metadata:     metadata,
	})

	return adjustment, nil
}
