package appointment

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	infraRepo "github.com/BruksfildServices01/barber-scheduler/internal/infra/repository"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
	ucSubscription "github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

type CompleteAppointment struct {
	db           *gorm.DB
	repo         domain.Repository
	paymentRepo  domainPayment.Repository
	audit        *audit.Dispatcher
	metrics      *ucMetrics.UpdateClientMetrics
	consumeCutUC *ucSubscription.ConsumeCut
}

func NewCompleteAppointment(
	db *gorm.DB,
	repo domain.Repository,
	paymentRepo domainPayment.Repository,
	audit *audit.Dispatcher,
	metrics *ucMetrics.UpdateClientMetrics,
	consumeCutUC *ucSubscription.ConsumeCut,
) *CompleteAppointment {
	return &CompleteAppointment{
		db:           db,
		repo:         repo,
		paymentRepo:  paymentRepo,
		audit:        audit,
		metrics:      metrics,
		consumeCutUC: consumeCutUC,
	}
}

type CompleteAppointmentInput struct {
	BarbershopID  uint
	BarberID      uint
	AppointmentID uint

	FinalAmountCents      *int64
	OperationalNote       string
	ConfirmNormalCharging bool
}

func (uc *CompleteAppointment) Execute(
	ctx context.Context,
	input CompleteAppointmentInput,
) (*models.Appointment, *models.AppointmentClosure, *ucSubscription.ConsumeCutResult, error) {

	barbershopID := input.BarbershopID
	barberID := input.BarberID
	appointmentID := input.AppointmentID

	appointmentRepoBase, ok := uc.repo.(*infraRepo.AppointmentGormRepository)
	if !ok {
		return nil, nil, nil, httperr.ErrBusiness("invalid_repository_impl")
	}

	var (
		ap               *models.Appointment
		closure          *models.AppointmentClosure
		consumeCutResult *ucSubscription.ConsumeCutResult
		referenceAmount  int64
	)

	err := uc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := appointmentRepoBase.WithTx(tx)

		// --------------------------------------------------
		// 1️⃣ Carrega appointment
		// --------------------------------------------------
		apLoaded, err := txRepo.GetAppointmentForBarber(
			ctx,
			barbershopID,
			appointmentID,
			barberID,
		)
		if err != nil || apLoaded == nil {
			return httperr.ErrBusiness("appointment_not_found")
		}
		ap = apLoaded

		if ap.BarbershopID == nil || *ap.BarbershopID != barbershopID {
			return httperr.ErrBusiness("invalid_barbershop")
		}

		if input.FinalAmountCents != nil && *input.FinalAmountCents < 0 {
			return httperr.ErrBusiness("invalid_final_amount")
		}

		// --------------------------------------------------
		// 2️⃣ Se aguardando pagamento → validar pagamento
		// --------------------------------------------------
		if ap.Status == models.AppointmentStatus(domain.StatusAwaitingPayment) {
			payment, err := uc.paymentRepo.GetByAppointmentID(ctx, barbershopID, ap.ID)
			if err != nil {
				return err
			}
			if payment == nil {
				return httperr.ErrBusiness("appointment_payment_not_found")
			}
			if payment.Status != models.PaymentStatus(domainPayment.StatusPaid) {
				return httperr.ErrBusiness("appointment_payment_not_paid")
			}
		}

		// --------------------------------------------------
		// 3️⃣ Consumir franquia de assinatura ANTES de concluir (best effort)
		// --------------------------------------------------
		if ap.ClientID != nil &&
			ap.BarberProductID != nil &&
			uc.consumeCutUC != nil {

			result, err := uc.consumeCutUC.Execute(
				ctx,
				barbershopID,
				*ap.ClientID,
				*ap.BarberProductID,
			)
			if err == nil {
				consumeCutResult = result
			}
		}

		// --------------------------------------------------
		// 4️⃣ Regra de domínio (complete)
		// --------------------------------------------------
		now := time.Now().UTC()

		if err := domain.Complete(ap, now); err != nil {
			return err
		}

		// --------------------------------------------------
		// 5️⃣ Persistir appointment
		// --------------------------------------------------
		if err := txRepo.UpdateAppointment(ctx, ap); err != nil {
			return err
		}

		// --------------------------------------------------
		// 6️⃣ Montar e persistir closure
		// --------------------------------------------------
		var serviceID *uint
		var serviceName string

		if ap.BarberProduct != nil {
			serviceID = &ap.BarberProduct.ID
			serviceName = ap.BarberProduct.Name
			referenceAmount = ap.BarberProduct.Price
		}

		subscriptionCovered := false
		requiresNormalCharging := true

		var subscriptionConsumeStatus *string
		var subscriptionPlanID *uint

		if consumeCutResult != nil {
			status := string(consumeCutResult.Status)
			subscriptionConsumeStatus = &status
			subscriptionPlanID = consumeCutResult.PlanID

			if status == "consumed" {
				subscriptionCovered = true
				requiresNormalCharging = false
			}
		}

		closure = &models.AppointmentClosure{
			AppointmentID:             ap.ID,
			BarbershopID:              barbershopID,
			ServiceID:                 serviceID,
			ServiceName:               serviceName,
			ReferenceAmountCents:      referenceAmount,
			FinalAmountCents:          input.FinalAmountCents,
			SubscriptionConsumeStatus: subscriptionConsumeStatus,
			SubscriptionPlanID:        subscriptionPlanID,
			SubscriptionCovered:       subscriptionCovered,
			RequiresNormalCharging:    requiresNormalCharging,
			ConfirmNormalCharging:     input.ConfirmNormalCharging,
			OperationalNote:           input.OperationalNote,
		}

		if err := txRepo.SaveAppointmentClosure(ctx, closure); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, nil, nil, err
	}

	// --------------------------------------------------
	// 7️⃣ Auditoria
	// --------------------------------------------------
	metadata := map[string]any{}

	if input.FinalAmountCents != nil {
		metadata["final_amount_cents"] = *input.FinalAmountCents
	}

	if input.OperationalNote != "" {
		metadata["operational_note"] = input.OperationalNote
	}

	metadata["confirm_normal_charging"] = input.ConfirmNormalCharging

	if ap != nil && ap.BarberProduct != nil {
		metadata["service_reference_price"] = ap.BarberProduct.Price
		metadata["service_id"] = ap.BarberProduct.ID
		metadata["service_name"] = ap.BarberProduct.Name
	}

	if consumeCutResult != nil {
		metadata["subscription_consume_status"] = consumeCutResult.Status
		if consumeCutResult.PlanID != nil {
			metadata["subscription_plan_id"] = *consumeCutResult.PlanID
		}
	}

	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		UserID:       &barberID,
		Action:       "appointment_completed",
		Entity:       "appointment",
		EntityID:     &ap.ID,
		Metadata:     metadata,
	})

	// --------------------------------------------------
	// 8️⃣ Métricas (best effort)
	// --------------------------------------------------
	if ap.ClientID != nil {
		_ = uc.metrics.Execute(ctx, ucMetrics.UpdateClientMetricsInput{
			BarbershopID: barbershopID,
			ClientID:     *ap.ClientID,
			EventType:    ucMetrics.EventAppointmentCompleted,
			OccurredAt:   time.Now().UTC(),
			Amount:       referenceAmount,
		})
	}

	return ap, closure, consumeCutResult, nil
}
