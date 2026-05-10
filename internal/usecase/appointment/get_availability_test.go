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

	// ── Novos testes: override e correções de conflito ────────────────────────

	t.Run("override expande dia normalmente fechado → mostra slots", func(t *testing.T) {
		// Working hours para o weekday: inativo (folga semanal)
		whInactive := &models.WorkingHours{Active: false}
		// Override abre o dia específico
		override := &models.ScheduleOverride{Closed: false, StartTime: "10:00", EndTime: "17:00"}

		repo := &mockRepo{shop: shop, product: product60min, workingHours: whInactive, override: override}
		uc := NewGetAvailability(repo)

		slots, err := uc.Execute(ctx, input(baseDate, 1))
		if err != nil {
			t.Fatalf("inesperado erro: %v", err)
		}
		// 10:00-17:00 com 60min → slots 10, 11, 12, 13, 14, 15, 16 = 7 slots
		if len(slots) != 7 {
			t.Errorf("override expande fechado: esperado 7 slots, obtido %d: %v", len(slots), slots)
		}
		if len(slots) > 0 && slots[0].Start != "10:00" {
			t.Errorf("primeiro slot esperado 10:00, obtido %s", slots[0].Start)
		}
	})

	t.Run("override herda almoço do working hours original", func(t *testing.T) {
		// Working hours com almoço; override muda apenas o horário (sem almoço próprio)
		whWithLunch := &models.WorkingHours{
			Active:     true,
			StartTime:  "09:00",
			EndTime:    "18:00",
			LunchStart: "12:00",
			LunchEnd:   "13:00",
		}
		// Override muda o horário mas deve herdar o almoço
		override := &models.ScheduleOverride{Closed: false, StartTime: "10:00", EndTime: "18:00"}

		repo := &mockRepo{shop: shop, product: product60min, workingHours: whWithLunch, override: override}
		uc := NewGetAvailability(repo)

		slots, err := uc.Execute(ctx, input(baseDate, 1))
		if err != nil {
			t.Fatalf("inesperado erro: %v", err)
		}

		// Override 10:00-18:00 com almoço herdado 12:00-13:00 → slots 10,11 + 13,14,15,16,17 = 7
		if len(slots) != 7 {
			t.Errorf("override com almoço herdado: esperado 7 slots, obtido %d: %v", len(slots), slots)
		}
		for _, s := range slots {
			if s.Start == "12:00" {
				t.Error("slot 12:00 não deve aparecer — almoço herdado do working hours original")
			}
		}
	})

	t.Run("appointment que começa antes do expediente e termina dentro bloqueia slot", func(t *testing.T) {
		// Cenário: encaixe interno criado às 08:30-09:30 (antes da abertura)
		// Após correção de ListAppointmentsForDay com overlap real, este appointment
		// deve ser incluído na lista e bloquear o slot 09:00-10:00.
		apBefore := time.Date(2030, 1, 7, 8, 30, 0, 0, loc) // 08:30 — antes do expediente
		apEnd := time.Date(2030, 1, 7, 9, 30, 0, 0, loc)     // 09:30 — dentro do expediente
		earlyAppointment := []models.Appointment{
			{StartTime: apBefore, EndTime: apEnd, Status: models.AppointmentStatusScheduled},
		}

		repo := &mockRepo{
			shop:         shop,
			product:      product60min,
			workingHours: wh9to18,
			appointments: earlyAppointment,
		}
		uc := NewGetAvailability(repo)

		slots, err := uc.Execute(ctx, input(baseDate, 1))
		if err != nil {
			t.Fatalf("inesperado erro: %v", err)
		}

		// Slot 09:00-10:00 deve estar bloqueado (appointment 08:30-09:30 se sobrepõe)
		for _, s := range slots {
			if s.Start == "09:00" {
				t.Error("slot 09:00 não deve aparecer — appointment 08:30-09:30 se sobrepõe")
			}
		}
		// Slot 10:00-11:00 deve estar disponível (fora do overlap do appointment)
		found10 := false
		for _, s := range slots {
			if s.Start == "10:00" {
				found10 = true
			}
		}
		if !found10 {
			t.Error("slot 10:00 deve estar disponível — appointment 08:30-09:30 não conflita")
		}
	})

	t.Run("conflito parcial no meio do intervalo de 60min é detectado", func(t *testing.T) {
		// Appointment de 30min às 10:30 conflita com slot 10:00-11:00 (meio do intervalo)
		apMid := time.Date(2030, 1, 7, 10, 30, 0, 0, loc)
		apMidEnd := time.Date(2030, 1, 7, 11, 0, 0, 0, loc)
		midAppointment := []models.Appointment{
			{StartTime: apMid, EndTime: apMidEnd, Status: models.AppointmentStatusScheduled},
		}

		repo := &mockRepo{
			shop:         shop,
			product:      product60min,
			workingHours: wh9to18,
			appointments: midAppointment,
		}
		uc := NewGetAvailability(repo)

		slots, err := uc.Execute(ctx, input(baseDate, 1))
		if err != nil {
			t.Fatalf("inesperado erro: %v", err)
		}

		// Slot 10:00-11:00 deve estar bloqueado (10:30-11:00 conflita com o slot)
		for _, s := range slots {
			if s.Start == "10:00" {
				t.Error("slot 10:00 não deve aparecer — appointment 10:30-11:00 conflita")
			}
		}
	})

	t.Run("dois appointments contíguos com tolerância: apIdx loop verifica ambos", func(t *testing.T) {
		// Com tolerância de 15min, pode existir A(09:50-10:05) e B(10:00-11:00).
		// O apIdx original verificava apenas appointments[apIdx] e podia perder B.
		shopTol := &models.Barbershop{
			ID: 1, Timezone: "America/Sao_Paulo",
			MinAdvanceMinutes: 0, ScheduleToleranceMinutes: 15,
		}
		apA := time.Date(2030, 1, 7, 9, 50, 0, 0, loc)
		apAEnd := time.Date(2030, 1, 7, 10, 5, 0, 0, loc)
		apB := time.Date(2030, 1, 7, 10, 0, 0, 0, loc)
		apBEnd := time.Date(2030, 1, 7, 11, 0, 0, 0, loc)
		twoAppointments := []models.Appointment{
			{StartTime: apA, EndTime: apAEnd, Status: models.AppointmentStatusScheduled},
			{StartTime: apB, EndTime: apBEnd, Status: models.AppointmentStatusScheduled},
		}

		repo := &mockRepo{
			shop:         shopTol,
			product:      product60min,
			workingHours: wh9to18,
			appointments: twoAppointments,
		}
		uc := NewGetAvailability(repo)

		slots, err := uc.Execute(ctx, input(baseDate, 1))
		if err != nil {
			t.Fatalf("inesperado erro: %v", err)
		}

		// Slot 10:00-11:00 (effectiveStart=10:15, effectiveEnd=10:45):
		// A(09:50-10:05): 10:05 ≤ effectiveStart(10:15) → não conflita mas não avança apIdx
		//   pois A.EndTime(10:05) > slotStart(10:00)
		// B(10:00-11:00): deve ser verificado e detectado como conflito
		for _, s := range slots {
			if s.Start == "10:00" {
				t.Error("slot 10:00 não deve aparecer — B(10:00-11:00) conflita mesmo com A não conflitando")
			}
		}
	})
}
