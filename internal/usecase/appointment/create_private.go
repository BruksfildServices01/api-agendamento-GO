package appointment

import (
	"context"
	"errors"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/audit"
	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	domainPayment "github.com/BruksfildServices01/barber-scheduler/internal/domain/paymentconfig"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/infra/idempotency"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
	paymentconfig "github.com/BruksfildServices01/barber-scheduler/internal/usecase/paymentconfig"
	ucSubscription "github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

type CreatePrivateAppointmentInput struct {
	BarbershopID uint
	BarberID     uint

	ClientName  string
	ClientPhone string
	ClientEmail string

	ProductID uint

	Date           string
	Time           string
	Notes          string
	IdempotencyKey string
}

type CreatePrivateAppointment struct {
	repo              domain.Repository
	audit             *audit.Dispatcher
	paymentPolicy     *paymentconfig.ResolveBookingPaymentPolicy
	metrics           *ucMetrics.UpdateClientMetrics
	getCategoryUC     *ucMetrics.GetClientCategory
	getSubscriptionUC *ucSubscription.GetActiveSubscription
	idempotency       idempotency.Store
}

func NewCreatePrivateAppointment(
	repo domain.Repository,
	audit *audit.Dispatcher,
	paymentPolicy *paymentconfig.ResolveBookingPaymentPolicy,
	metrics *ucMetrics.UpdateClientMetrics,
	getCategoryUC *ucMetrics.GetClientCategory,
	getSubscriptionUC *ucSubscription.GetActiveSubscription,
	idempotency idempotency.Store,
) *CreatePrivateAppointment {
	return &CreatePrivateAppointment{
		repo:              repo,
		audit:             audit,
		paymentPolicy:     paymentPolicy,
		metrics:           metrics,
		getCategoryUC:     getCategoryUC,
		getSubscriptionUC: getSubscriptionUC,
		idempotency:       idempotency,
	}
}

func (uc *CreatePrivateAppointment) Execute(
	ctx context.Context,
	in CreatePrivateAppointmentInput,
) (*models.Appointment, error) {

	// --------------------------------------------------
	// 1) Barbearia (timezone é fonte da verdade)
	// --------------------------------------------------
	shop, err := uc.repo.GetBarbershopByID(ctx, in.BarbershopID)
	if err != nil {
		return nil, err
	}
	if shop == nil {
		return nil, httperr.ErrBusiness("barbershop_not_found")
	}

	loc := timezone.Location(shop.Timezone)

	// --------------------------------------------------
	// 2) Política de pagamento
	// --------------------------------------------------
	policy, err := uc.paymentPolicy.Execute(ctx, in.BarbershopID)
	if err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 3) Data / Hora (sempre interpretada no timezone da barbearia)
	// --------------------------------------------------
	start, err := time.ParseInLocation(
		"2006-01-02 15:04",
		in.Date+" "+in.Time,
		loc,
	)
	if err != nil {
		return nil, httperr.ErrBusiness("invalid_date_or_time")
	}

	// Antecedência mínima
	minAdvance := shop.MinAdvanceMinutes
	if minAdvance <= 0 {
		minAdvance = 120
	}

	nowLocal := time.Now().In(loc)
	if start.Before(nowLocal.Add(time.Duration(minAdvance) * time.Minute)) {
		return nil, httperr.ErrBusiness("too_soon")
	}

	// --------------------------------------------------
	// 4) Produto
	// --------------------------------------------------
	product, err := uc.repo.GetProduct(ctx, in.BarbershopID, in.ProductID)
	if err != nil || product == nil {
		return nil, httperr.ErrBusiness("product_not_found")
	}

	end := start.Add(time.Duration(product.DurationMin) * time.Minute)

	// --------------------------------------------------
	// 5) Horário de trabalho (timezone-safe)
	// --------------------------------------------------
	startLocal := start.In(loc)
	endLocal := end.In(loc)

	weekday := int(startLocal.Weekday())

	wh, err := uc.repo.GetWorkingHours(ctx, in.BarbershopID, in.BarberID, weekday)
	if err != nil {
		return nil, err
	}

	if wh == nil || !wh.Active || wh.StartTime == "" || wh.EndTime == "" {
		return nil, httperr.ErrBusiness("outside_working_hours")
	}

	parseHM := func(hm string) time.Time {
		t, _ := time.Parse("15:04", hm)
		return time.Date(
			startLocal.Year(), startLocal.Month(), startLocal.Day(),
			t.Hour(), t.Minute(), 0, 0,
			loc,
		)
	}

	workStart := parseHM(wh.StartTime)
	workEnd := parseHM(wh.EndTime)

	if startLocal.Before(workStart) || endLocal.After(workEnd) {
		return nil, httperr.ErrBusiness("outside_working_hours")
	}

	if wh.LunchStart != "" && wh.LunchEnd != "" {
		lunchStart := parseHM(wh.LunchStart)
		lunchEnd := parseHM(wh.LunchEnd)

		if startLocal.Before(lunchEnd) && endLocal.After(lunchStart) {
			return nil, httperr.ErrBusiness("outside_working_hours")
		}
	}

	// --------------------------------------------------
	// 6) Cliente
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
	// 7) Conflito de horário (com tolerância configurada)
	// --------------------------------------------------
	// A tolerância permite sobreposição de até T minutos em cada extremidade,
	// espelhando a mesma lógica usada em get_availability.go.
	conflictStart := start
	conflictEnd := end
	if shop.ScheduleToleranceMinutes > 0 {
		tol := time.Duration(shop.ScheduleToleranceMinutes) * time.Minute
		conflictStart = start.Add(tol)
		conflictEnd = end.Add(-tol)
	}
	if conflictStart.Before(conflictEnd) {
		if err := uc.repo.AssertNoTimeConflict(
			ctx,
			in.BarbershopID,
			in.BarberID,
			conflictStart,
			conflictEnd,
		); err != nil {
			return nil, err
		}
	}

	// --------------------------------------------------
	// 8) Categoria CRM do cliente (comportamental)
	// --------------------------------------------------
	category, err := uc.getCategoryUC.Execute(
		ctx,
		in.BarbershopID,
		client.ID,
	)
	if err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 9) Assinatura ativa + cobertura do serviço
	// --------------------------------------------------
	hasActiveSubscription := false
	subscriptionCoversService := false

	if uc.getSubscriptionUC != nil {
		sub, err := uc.getSubscriptionUC.Execute(
			ctx,
			in.BarbershopID,
			client.ID,
		)
		if err != nil {
			return nil, err
		}

		if sub != nil {
			hasActiveSubscription = true

			if sub.Plan != nil {
				for _, allowedServiceID := range sub.Plan.ServiceIDs {
					if allowedServiceID == product.ID {
						subscriptionCoversService = true
						break
					}
				}
			}
		}
	}

	// --------------------------------------------------
	// 10) Regra final de cobrança
	// --------------------------------------------------
	requirement := policy.CategoryPolicies.RequirementFor(
		category,
		policy.DefaultRequirement,
	)

	if hasActiveSubscription && subscriptionCoversService {
		requirement = domainPayment.PaymentOptional
	}

	initialStatus := domain.StatusScheduled
	if requirement == domainPayment.PaymentMandatory {
		initialStatus = domain.StatusAwaitingPayment
	}

	// --------------------------------------------------
	// 11) Idempotência (checa antes, grava só no sucesso)
	// --------------------------------------------------
	idempotencyStorageKey := ""
	if uc.idempotency != nil && in.IdempotencyKey != "" {
		idempotencyStorageKey = "appointment:create:" + in.IdempotencyKey

		exists, err := uc.idempotency.Exists(ctx, idempotencyStorageKey)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, httperr.ErrBusiness("duplicate_request")
		}
	}

	// --------------------------------------------------
	// 12) Criar Appointment
	// --------------------------------------------------
	barbershopID := in.BarbershopID
	barberID := in.BarberID
	clientID := client.ID
	productID := product.ID

	status := models.AppointmentStatus(initialStatus)

	ap := &models.Appointment{
		BarbershopID:    &barbershopID,
		BarberID:        &barberID,
		ClientID:        &clientID,
		BarberProductID: &productID,
		StartTime:       start,
		EndTime:         end,
		Status:          status,
		CreatedBy:       models.CreatedByClient,
		PaymentIntent:   models.PaymentIntentPayLater,
		Notes:           in.Notes,
	}

	if err := uc.repo.CreateAppointment(ctx, ap); err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 13) Persistir chave de idempotência após sucesso real
	// --------------------------------------------------
	if idempotencyStorageKey != "" {
		if err := uc.idempotency.Save(ctx, idempotencyStorageKey); err != nil {
			if errors.Is(err, idempotency.ErrDuplicateRequest) {
				return nil, httperr.ErrBusiness("duplicate_request")
			}
			return nil, err
		}
	}

	// --------------------------------------------------
	// 14) Métricas
	// --------------------------------------------------
	_ = uc.metrics.Execute(ctx, ucMetrics.UpdateClientMetricsInput{
		BarbershopID: in.BarbershopID,
		ClientID:     client.ID,
		EventType:    ucMetrics.EventAppointmentCreated,
		OccurredAt:   time.Now().UTC(),
		Amount:       0,
	})

	return ap, nil
}
