package appointment

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	orderDomain "github.com/BruksfildServices01/barber-scheduler/internal/domain/order"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/payment"
	productDomain "github.com/BruksfildServices01/barber-scheduler/internal/domain/product"
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
	orderRepo    *infraRepo.OrderGormRepository
	productRepo  *infraRepo.ProductGormRepository
	audit        *audit.Dispatcher
	metrics      *ucMetrics.UpdateClientMetrics
	consumeCutUC *ucSubscription.ConsumeCut
}

func NewCompleteAppointment(
	db *gorm.DB,
	repo domain.Repository,
	paymentRepo domainPayment.Repository,
	orderRepo *infraRepo.OrderGormRepository,
	productRepo *infraRepo.ProductGormRepository,
	audit *audit.Dispatcher,
	metrics *ucMetrics.UpdateClientMetrics,
	consumeCutUC *ucSubscription.ConsumeCut,
) *CompleteAppointment {
	return &CompleteAppointment{
		db:           db,
		repo:         repo,
		paymentRepo:  paymentRepo,
		orderRepo:    orderRepo,
		productRepo:  productRepo,
		audit:        audit,
		metrics:      metrics,
		consumeCutUC: consumeCutUC,
	}
}

// ClosureItemInput is a product sold during the appointment (venda adicional).
type ClosureItemInput struct {
	ProductID uint
	Quantity  int
}

type CompleteAppointmentInput struct {
	BarbershopID  uint
	BarberID      uint
	AppointmentID uint

	// Serviço realizado — se nil, usa o serviço agendado originalmente.
	ActualServiceID *uint

	// Valor final cobrado — se nil, usa o preço de referência do serviço.
	FinalAmountCents *int64

	// Venda adicional de produtos durante o atendimento.
	AdditionalItems []ClosureItemInput

	// Forma de pagamento real: "cash" | "card" | "pix" | "subscription".
	PaymentMethod string

	// O item previsto (suggestion) foi removido/não utilizado.
	SuggestionRemoved bool

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

		apLoaded, err := txRepo.GetAppointmentForBarber(ctx, barbershopID, appointmentID, barberID)
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

		// Resolves the actual service: use ActualServiceID if provided, else the scheduled one.
		actualServiceID := ap.BarberProductID
		actualServiceName := ""
		if input.ActualServiceID != nil {
			actualServiceID = input.ActualServiceID
		}

		// Load actual service details for reference price and subscription check.
		if ap.BarberProduct != nil && (input.ActualServiceID == nil || *input.ActualServiceID == ap.BarberProduct.ID) {
			actualServiceName = ap.BarberProduct.Name
			referenceAmount = ap.BarberProduct.Price
		} else if actualServiceID != nil {
			var svc models.BarbershopService
			if err := tx.WithContext(ctx).
				Where("id = ? AND barbershop_id = ?", *actualServiceID, barbershopID).
				First(&svc).Error; err != nil {
				return httperr.ErrBusiness("actual_service_not_found")
			}
			actualServiceName = svc.Name
			referenceAmount = svc.Price
		}

		// Consume subscription using the ACTUAL service performed.
		if ap.ClientID != nil && actualServiceID != nil && uc.consumeCutUC != nil {
			result, err := uc.consumeCutUC.Execute(ctx, barbershopID, *ap.ClientID, *actualServiceID)
			if err != nil {
				return err
			}
			consumeCutResult = result
		}

		now := time.Now().UTC()

		if err := domain.Complete(ap, now); err != nil {
			return err
		}

		if err := txRepo.UpdateAppointment(ctx, ap); err != nil {
			return err
		}

		subscriptionCovered := false
		requiresNormalCharging := true

		var subscriptionConsumeStatus *string
		var subscriptionPlanID *uint

		if consumeCutResult != nil {
			status := string(consumeCutResult.Status)
			subscriptionConsumeStatus = &status
			subscriptionPlanID = consumeCutResult.PlanID

			switch consumeCutResult.Status {
			case ucSubscription.ConsumeCutStatusConsumed:
				subscriptionCovered = true
				requiresNormalCharging = false
			}
		}

		if requiresNormalCharging && !input.ConfirmNormalCharging {
			return httperr.ErrBusiness("normal_charging_confirmation_required")
		}

		// Venda adicional — cria Order dentro da mesma transação.
		var additionalOrderID *uint
		if len(input.AdditionalItems) > 0 {
			txOrderRepo := uc.orderRepo.WithTx(tx)
			txProductRepo := uc.productRepo.WithTx(tx)

			order := orderDomain.New(barbershopID, ap.ClientID)

			for _, item := range input.AdditionalItems {
				product, err := txProductRepo.GetByID(ctx, barbershopID, item.ProductID)
				if err != nil {
					return err
				}
				if product == nil {
					return productDomain.ErrProductNotFound
				}
				if err := order.AddItem(product.ID, product.Name, item.Quantity, product.Price); err != nil {
					return err
				}
			}

			if err := order.Validate(); err != nil {
				return err
			}

			if err := txOrderRepo.Create(ctx, order); err != nil {
				return err
			}

			additionalOrderID = &order.ID
		}

		closure = &models.AppointmentClosure{
			AppointmentID:             ap.ID,
			BarbershopID:              barbershopID,
			ServiceID:                 ap.BarberProductID,
			ServiceName:               func() string {
				if ap.BarberProduct != nil {
					return ap.BarberProduct.Name
				}
				return ""
			}(),
			ReferenceAmountCents:      referenceAmount,
			FinalAmountCents:          input.FinalAmountCents,
			SubscriptionConsumeStatus: subscriptionConsumeStatus,
			SubscriptionPlanID:        subscriptionPlanID,
			SubscriptionCovered:       subscriptionCovered,
			RequiresNormalCharging:    requiresNormalCharging,
			ConfirmNormalCharging:     input.ConfirmNormalCharging,
			OperationalNote:           input.OperationalNote,
			ActualServiceID:           actualServiceID,
			ActualServiceName:         actualServiceName,
			PaymentMethod:             input.PaymentMethod,
			AdditionalOrderID:         additionalOrderID,
			SuggestionRemoved:         input.SuggestionRemoved,
		}

		if err := txRepo.SaveAppointmentClosure(ctx, closure); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, nil, nil, err
	}

	// Audit
	metadata := map[string]any{
		"confirm_normal_charging": input.ConfirmNormalCharging,
		"suggestion_removed":      input.SuggestionRemoved,
	}

	if input.FinalAmountCents != nil {
		metadata["final_amount_cents"] = *input.FinalAmountCents
	}
	if input.OperationalNote != "" {
		metadata["operational_note"] = input.OperationalNote
	}
	if input.PaymentMethod != "" {
		metadata["payment_method"] = input.PaymentMethod
	}
	if ap != nil && ap.BarberProduct != nil {
		metadata["scheduled_service_id"] = ap.BarberProduct.ID
		metadata["scheduled_service_name"] = ap.BarberProduct.Name
		metadata["service_reference_price"] = ap.BarberProduct.Price
	}
	if closure != nil && closure.ActualServiceID != nil {
		metadata["actual_service_id"] = *closure.ActualServiceID
		metadata["actual_service_name"] = closure.ActualServiceName
	}
	if consumeCutResult != nil {
		metadata["subscription_consume_status"] = consumeCutResult.Status
		if consumeCutResult.PlanID != nil {
			metadata["subscription_plan_id"] = *consumeCutResult.PlanID
		}
	}
	if closure != nil && closure.AdditionalOrderID != nil {
		metadata["additional_order_id"] = *closure.AdditionalOrderID
	}

	uc.audit.Dispatch(audit.Event{
		BarbershopID: barbershopID,
		UserID:       &barberID,
		Action:       "appointment_completed",
		Entity:       "appointment",
		EntityID:     &ap.ID,
		Metadata:     metadata,
	})

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
