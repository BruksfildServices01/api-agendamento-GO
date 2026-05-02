package appointment

import (
	"context"
	"testing"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
)

func TestGetAvailability(t *testing.T) {
	ctx := context.Background()

	// Referência: segunda-feira às 09:00 (horário de Brasília → UTC-3)
	// time.Now() pode interferir nos testes de antecedência, então usamos uma data no futuro distante.
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	// Data no futuro: segunda-feira 2030-01-07 — garante que slots não sejam filtrados por antecedência
	baseDate := time.Date(2030, 1, 7, 0, 0, 0, 0, loc)

	shop := &models.Barbershop{
		ID:                       1,
		Timezone:                 "America/Sao_Paulo",
		MinAdvanceMinutes:        0,
		ScheduleToleranceMinutes: 0,
	}

	product60min := &models.BarbershopService{ID: 1, DurationMin: 60}
	product30min := &models.BarbershopService{ID: 2, DurationMin: 30}

	wh9to18 := &models.WorkingHours{
		Active:    true,
		StartTime: "09:00",
		EndTime:   "18:00",
	}

	wh9to18withLunch := &models.WorkingHours{
		Active:     true,
		StartTime:  "09:00",
		EndTime:    "18:00",
		LunchStart: "12:00",
		LunchEnd:   "13:00",
	}

	input := func(date time.Time, productID uint) domain.AvailabilityInput {
		return domain.AvailabilityInput{
			BarbershopID: 1,
			BarberID:     1,
			ProductID:    productID,
			Date:         date,
		}
	}

	t.Run("dia sem agendamentos retorna todos os slots do horário de trabalho", func(t *testing.T) {
		repo := &mockRepo{shop: shop, product: product60min, workingHours: wh9to18}
		uc := NewGetAvailability(repo)

		slots, err := uc.Execute(ctx, input(baseDate, 1))
		if err != nil {
			t.Fatalf("inesperado erro: %v", err)
		}

		// 09:00-18:00 com serviço de 60min → 9 slots: 09, 10, 11, 12, 13, 14, 15, 16, 17
		if len(slots) != 9 {
			t.Errorf("esperado 9 slots, obtido %d", len(slots))
		}
		if slots[0].Start != "09:00" {
			t.Errorf("primeiro slot esperado 09:00, obtido %s", slots[0].Start)
		}
		if slots[len(slots)-1].Start != "17:00" {
			t.Errorf("último slot esperado 17:00, obtido %s", slots[len(slots)-1].Start)
		}
	})

	t.Run("agendamento existente bloqueia slot sobreposto", func(t *testing.T) {
		// Agendamento das 10:00 às 11:00
		apStart := time.Date(2030, 1, 7, 10, 0, 0, 0, loc)
		apEnd := apStart.Add(60 * time.Minute)
		existing := []models.Appointment{{StartTime: apStart, EndTime: apEnd, Status: models.AppointmentStatusScheduled}}

		repo := &mockRepo{shop: shop, product: product60min, workingHours: wh9to18, appointments: existing}
		uc := NewGetAvailability(repo)

		slots, err := uc.Execute(ctx, input(baseDate, 1))
		if err != nil {
			t.Fatalf("inesperado erro: %v", err)
		}

		// 10:00 deve estar bloqueado → 8 slots
		if len(slots) != 8 {
			t.Errorf("esperado 8 slots, obtido %d: %v", len(slots), slots)
		}
		for _, s := range slots {
			if s.Start == "10:00" {
				t.Error("slot 10:00 não deveria estar disponível")
			}
		}
	})

	t.Run("lunch break remove slots sobrepostos", func(t *testing.T) {
		repo := &mockRepo{shop: shop, product: product60min, workingHours: wh9to18withLunch}
		uc := NewGetAvailability(repo)

		slots, err := uc.Execute(ctx, input(baseDate, 1))
		if err != nil {
			t.Fatalf("inesperado erro: %v", err)
		}

		// 09-18 com lunch 12-13 → slots 09,10,11 + 13,14,15,16,17 = 8 (slot 12:00 cobre 12-13, conflita com lunch)
		for _, s := range slots {
			if s.Start == "12:00" {
				t.Error("slot 12:00 não deveria estar disponível (lunch break)")
			}
		}
	})

	t.Run("tolerância não bloqueia slots adjacentes dentro do limite", func(t *testing.T) {
		shopWithTol := &models.Barbershop{
			ID:                       1,
			Timezone:                 "America/Sao_Paulo",
			MinAdvanceMinutes:        0,
			ScheduleToleranceMinutes: 5, // 5 min de tolerância
		}

		// Agendamento das 10:00 às 11:00 — com tolerância de 5min,
		// slot 09:00 (termina 10:00) não conflita porque effectiveEnd = 09:55 < 10:00
		apStart := time.Date(2030, 1, 7, 10, 0, 0, 0, loc)
		apEnd := apStart.Add(60 * time.Minute)
		existing := []models.Appointment{{StartTime: apStart, EndTime: apEnd, Status: models.AppointmentStatusScheduled}}

		repo := &mockRepo{shop: shopWithTol, product: product60min, workingHours: wh9to18, appointments: existing}
		uc := NewGetAvailability(repo)

		slots, err := uc.Execute(ctx, input(baseDate, 1))
		if err != nil {
			t.Fatalf("inesperado erro: %v", err)
		}

		// Com tolerância de 5min, slot 09:00 fica disponível pois 09:55 < 10:00
		var found09 bool
		for _, s := range slots {
			if s.Start == "09:00" {
				found09 = true
			}
		}
		if !found09 {
			t.Error("slot 09:00 deveria estar disponível com tolerância de 5 min")
		}
	})

	t.Run("override fechado retorna zero slots", func(t *testing.T) {
		closed := &models.ScheduleOverride{Closed: true}
		repo := &mockRepo{shop: shop, product: product60min, workingHours: wh9to18, override: closed}
		uc := NewGetAvailability(repo)

		slots, err := uc.Execute(ctx, input(baseDate, 1))
		if err != nil {
			t.Fatalf("inesperado erro: %v", err)
		}
		if len(slots) != 0 {
			t.Errorf("esperado 0 slots para dia fechado, obtido %d", len(slots))
		}
	})

	t.Run("override substitui horário padrão", func(t *testing.T) {
		// Override reduz horário para 14:00-16:00
		override := &models.ScheduleOverride{Closed: false, StartTime: "14:00", EndTime: "16:00"}
		repo := &mockRepo{shop: shop, product: product60min, workingHours: wh9to18, override: override}
		uc := NewGetAvailability(repo)

		slots, err := uc.Execute(ctx, input(baseDate, 1))
		if err != nil {
			t.Fatalf("inesperado erro: %v", err)
		}

		// 14:00-16:00 com serviço de 60min → slots 14:00 e 15:00
		if len(slots) != 2 {
			t.Errorf("esperado 2 slots com override, obtido %d: %v", len(slots), slots)
		}
	})

	t.Run("dia inativo retorna zero slots", func(t *testing.T) {
		whInactive := &models.WorkingHours{Active: false}
		repo := &mockRepo{shop: shop, product: product60min, workingHours: whInactive}
		uc := NewGetAvailability(repo)

		slots, err := uc.Execute(ctx, input(baseDate, 1))
		if err != nil {
			t.Fatalf("inesperado erro: %v", err)
		}
		if len(slots) != 0 {
			t.Errorf("esperado 0 slots para dia inativo, obtido %d", len(slots))
		}
	})

	t.Run("produto não encontrado retorna erro", func(t *testing.T) {
		repo := &mockRepo{shop: shop, product: nil, productErr: nil}
		uc := NewGetAvailability(repo)

		_, err := uc.Execute(ctx, input(baseDate, 99))
		if err == nil {
			t.Error("esperado erro para produto não encontrado")
		}
	})

	t.Run("slots com serviço de 30min", func(t *testing.T) {
		repo := &mockRepo{shop: shop, product: product30min, workingHours: wh9to18}
		uc := NewGetAvailability(repo)

		slots, err := uc.Execute(ctx, input(baseDate, 2))
		if err != nil {
			t.Fatalf("inesperado erro: %v", err)
		}

		// 09:00-18:00 com serviço de 30min → 18 slots
		if len(slots) != 18 {
			t.Errorf("esperado 18 slots para serviço de 30min, obtido %d", len(slots))
		}
	})
}
