package appointment

import (
	"context"
	"testing"
	"time"

	domainMetrics "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
	domainPaymentConfig "github.com/BruksfildServices01/barber-scheduler/internal/domain/paymentconfig"
	"github.com/BruksfildServices01/barber-scheduler/internal/apperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
	paymentconfig "github.com/BruksfildServices01/barber-scheduler/internal/usecase/paymentconfig"
)

// ── Mocks de dependências ────────────────────────────────────────────────────

type mockPaymentConfigRepo struct {
	cfg        *domainPaymentConfig.Config
	categories []domainPaymentConfig.CategoryPaymentPolicy
	err        error
}

func (r *mockPaymentConfigRepo) GetByBarbershopID(_ context.Context, _ uint) (*domainPaymentConfig.Config, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.cfg != nil {
		return r.cfg, nil
	}
	return &domainPaymentConfig.Config{DefaultRequirement: domainPaymentConfig.PaymentOptional}, nil
}

func (r *mockPaymentConfigRepo) ListCategoryPolicies(_ context.Context, _ uint) ([]domainPaymentConfig.CategoryPaymentPolicy, error) {
	return r.categories, nil
}

func (r *mockPaymentConfigRepo) UpsertCategoryPolicy(_ context.Context, _ uint, _ domainPaymentConfig.CategoryPaymentPolicy) error {
	return nil
}

func (r *mockPaymentConfigRepo) UpsertConfig(_ context.Context, _ *domainPaymentConfig.Config) error {
	return nil
}

func (r *mockPaymentConfigRepo) DeleteCategoryPolicies(_ context.Context, _ uint) error {
	return nil
}

type mockMetricsRepo struct{}

func (r *mockMetricsRepo) GetOrCreate(_ context.Context, _, _ uint) (*domainMetrics.ClientMetrics, error) {
	return &domainMetrics.ClientMetrics{}, nil
}

func (r *mockMetricsRepo) FindByBarbershop(_ context.Context, _ uint) ([]*domainMetrics.ClientMetrics, error) {
	return nil, nil
}

func (r *mockMetricsRepo) Save(_ context.Context, _ *domainMetrics.ClientMetrics) error {
	return nil
}

// ── buildCreateUC monta o use case com dependências mockadas ─────────────────

func buildCreateUC(
	repo *mockRepo,
	paymentConfigRepo domainPaymentConfig.Repository,
	paymentEnabled bool,
) *CreatePrivateAppointment {
	if paymentConfigRepo == nil {
		cfg := &domainPaymentConfig.Config{DefaultRequirement: domainPaymentConfig.PaymentOptional}
		if paymentEnabled {
			cfg.MPPublicKey = "pk_test"
			cfg.MPAccessToken = "at_test"
		}
		paymentConfigRepo = &mockPaymentConfigRepo{cfg: cfg}
	}

	policyUC := paymentconfig.NewResolveBookingPaymentPolicy(paymentConfigRepo)
	categoryUC := ucMetrics.NewGetClientCategory(&mockMetricsRepo{})
	// UpdateClientMetrics com nil repo/db — seguro porque client.ID=0 dispara early return
	metricsUC := ucMetrics.NewUpdateClientMetrics(nil, nil)

	return NewCreatePrivateAppointment(
		repo,
		nil, // audit: nunca usado em Execute
		policyUC,
		metricsUC,
		categoryUC,
		nil, // getSubscriptionUC: nil-checked no Execute
		nil, // reserveCutUC: nil-checked no Execute
		nil, // idempotency: nil-checked no Execute
	)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func futureDate(loc *time.Location) (string, string) {
	// Data no futuro distante para não ser filtrada por antecedência mínima
	d := time.Date(2030, 6, 10, 0, 0, 0, 0, loc)
	return d.Format("2006-01-02"), "10:00"
}

func defaultShop() *models.Barbershop {
	return &models.Barbershop{
		ID:                       1,
		Timezone:                 "America/Sao_Paulo",
		MinAdvanceMinutes:        0,
		ScheduleToleranceMinutes: 0,
	}
}

func defaultProduct() *models.BarbershopService {
	return &models.BarbershopService{ID: 1, DurationMin: 60}
}

func defaultWorkingHours() *models.WorkingHours {
	return &models.WorkingHours{Active: true, StartTime: "09:00", EndTime: "18:00"}
}

// client.ID=0 garante que UpdateClientMetrics retorna cedo (guard ClientID==0)
func zeroClient() *models.Client {
	return &models.Client{ID: 0}
}

func defaultInput(date, t string) CreatePrivateAppointmentInput {
	return CreatePrivateAppointmentInput{
		BarbershopID: 1,
		BarberID:     1,
		ClientName:   "Test Client",
		ClientPhone:  "11999999999",
		ProductID:    1,
		Date:         date,
		Time:         t,
	}
}

// ── Testes ───────────────────────────────────────────────────────────────────

func TestCreatePrivateAppointment(t *testing.T) {
	ctx := context.Background()
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	date, hr := futureDate(loc)

	t.Run("cria agendamento com sucesso", func(t *testing.T) {
		repo := &mockRepo{
			shop:         defaultShop(),
			product:      defaultProduct(),
			workingHours: defaultWorkingHours(),
			client:       zeroClient(),
		}
		uc := buildCreateUC(repo, nil, false)

		ap, err := uc.Execute(ctx, defaultInput(date, hr))
		if err != nil {
			t.Fatalf("inesperado erro: %v", err)
		}
		if ap == nil {
			t.Fatal("appointment não deveria ser nil")
		}
		if ap.Status != models.AppointmentStatusScheduled {
			t.Errorf("status esperado 'scheduled', obtido '%s'", ap.Status)
		}
	})

	t.Run("payment mandatory resulta em awaiting_payment", func(t *testing.T) {
		cfg := &domainPaymentConfig.Config{
			DefaultRequirement: domainPaymentConfig.PaymentMandatory,
			MPPublicKey:        "pk_test",
			MPAccessToken:      "at_test",
		}
		repo := &mockRepo{
			shop:         defaultShop(),
			product:      defaultProduct(),
			workingHours: defaultWorkingHours(),
			client:       zeroClient(),
		}
		uc := buildCreateUC(repo, &mockPaymentConfigRepo{cfg: cfg}, true)

		ap, err := uc.Execute(ctx, defaultInput(date, hr))
		if err != nil {
			t.Fatalf("inesperado erro: %v", err)
		}
		if ap.Status != models.AppointmentStatusAwaitingPayment {
			t.Errorf("status esperado 'awaiting_payment', obtido '%s'", ap.Status)
		}
	})

	t.Run("barbershop não encontrado retorna erro", func(t *testing.T) {
		repo := &mockRepo{shop: nil}
		uc := buildCreateUC(repo, nil, false)

		_, err := uc.Execute(ctx, defaultInput(date, hr))
		if !apperr.IsBusiness(err, "barbershop_not_found") {
			t.Errorf("esperado barbershop_not_found, obtido: %v", err)
		}
	})

	t.Run("produto não encontrado retorna erro", func(t *testing.T) {
		repo := &mockRepo{shop: defaultShop(), product: nil}
		uc := buildCreateUC(repo, nil, false)

		_, err := uc.Execute(ctx, defaultInput(date, hr))
		if !apperr.IsBusiness(err, "product_not_found") {
			t.Errorf("esperado product_not_found, obtido: %v", err)
		}
	})

	t.Run("data no passado retorna too_soon", func(t *testing.T) {
		shop := defaultShop()
		shop.MinAdvanceMinutes = 120

		repo := &mockRepo{shop: shop, product: defaultProduct()}
		uc := buildCreateUC(repo, nil, false)

		// Ontem
		past := time.Now().In(loc).Add(-24 * time.Hour)
		_, err := uc.Execute(ctx, defaultInput(past.Format("2006-01-02"), "10:00"))
		if !apperr.IsBusiness(err, "too_soon") {
			t.Errorf("esperado too_soon, obtido: %v", err)
		}
	})

	t.Run("horário fora do expediente retorna outside_working_hours", func(t *testing.T) {
		repo := &mockRepo{
			shop:         defaultShop(),
			product:      defaultProduct(),
			workingHours: &models.WorkingHours{Active: false},
		}
		uc := buildCreateUC(repo, nil, false)

		_, err := uc.Execute(ctx, defaultInput(date, hr))
		if !apperr.IsBusiness(err, "outside_working_hours") {
			t.Errorf("esperado outside_working_hours, obtido: %v", err)
		}
	})

	t.Run("horário durante almoço retorna outside_working_hours", func(t *testing.T) {
		wh := &models.WorkingHours{
			Active:     true,
			StartTime:  "09:00",
			EndTime:    "18:00",
			LunchStart: "12:00",
			LunchEnd:   "13:00",
		}
		repo := &mockRepo{
			shop:         defaultShop(),
			product:      defaultProduct(),
			workingHours: wh,
			client:       zeroClient(),
		}
		uc := buildCreateUC(repo, nil, false)

		// Horário no almoço: 12:00-13:00 com serviço de 60min → conflito com lunch
		_, err := uc.Execute(ctx, defaultInput(date, "12:00"))
		if !apperr.IsBusiness(err, "outside_working_hours") {
			t.Errorf("esperado outside_working_hours no almoço, obtido: %v", err)
		}
	})

	t.Run("conflito de horário retorna time_conflict", func(t *testing.T) {
		repo := &mockRepo{
			shop:         defaultShop(),
			product:      defaultProduct(),
			workingHours: defaultWorkingHours(),
			client:       zeroClient(),
			conflictErr:  apperr.ErrBusiness("time_conflict"),
		}
		uc := buildCreateUC(repo, nil, false)

		_, err := uc.Execute(ctx, defaultInput(date, hr))
		if !apperr.IsBusiness(err, "time_conflict") {
			t.Errorf("esperado time_conflict, obtido: %v", err)
		}
	})

	t.Run("data/hora inválida retorna invalid_date_or_time", func(t *testing.T) {
		repo := &mockRepo{shop: defaultShop()}
		uc := buildCreateUC(repo, nil, false)

		_, err := uc.Execute(ctx, defaultInput("not-a-date", "not-a-time"))
		if !apperr.IsBusiness(err, "invalid_date_or_time") {
			t.Errorf("esperado invalid_date_or_time, obtido: %v", err)
		}
	})
}
