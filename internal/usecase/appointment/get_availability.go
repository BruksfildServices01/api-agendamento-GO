package appointment

import (
	"context"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
	"github.com/BruksfildServices01/barber-scheduler/internal/models"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
)

type GetAvailability struct {
	repo domain.Repository
}

func NewGetAvailability(repo domain.Repository) *GetAvailability {
	return &GetAvailability{repo: repo}
}

func (uc *GetAvailability) Execute(
	ctx context.Context,
	in domain.AvailabilityInput,
) ([]domain.TimeSlot, error) {

	// 1 & 2) Paralelo: barbearia e produto não têm dependência entre si.
	// O receive no canal acontece-after o send da goroutine, garantindo
	// visibilidade de memória de shop e product sem sincronização adicional.
	var (
		shop    *models.Barbershop
		product *models.BarbershopService
	)
	shopCh    := make(chan error, 1)
	productCh := make(chan error, 1)

	go func() {
		var err error
		shop, err = uc.repo.GetBarbershopByID(ctx, in.BarbershopID)
		shopCh <- err
	}()
	go func() {
		var err error
		product, err = uc.repo.GetProduct(ctx, in.BarbershopID, in.ProductID)
		productCh <- err
	}()

	// Drena os dois canais antes de checar erros — evita goroutine leak e
	// garante que ambas as atribuições sejam visíveis após os receives.
	shopErr    := <-shopCh
	productErr := <-productCh

	if shopErr != nil {
		return nil, shopErr
	}
	if shop == nil {
		return nil, httperr.ErrBusiness("barbershop_not_found")
	}
	if productErr != nil || product == nil {
		return nil, httperr.ErrBusiness("product_not_found")
	}

	loc := timezone.Location(shop.Timezone)
	dateLocal := in.Date.In(loc)

	// 3) Working hours do weekday LOCAL
	weekday := int(dateLocal.Weekday())

	wh, err := uc.repo.GetWorkingHours(ctx, in.BarbershopID, in.BarberID, weekday)
	if err != nil || wh == nil || !wh.Active {
		return []domain.TimeSlot{}, nil
	}

	parseHM := func(hm string) time.Time {
		t, _ := time.Parse("15:04", hm)
		return time.Date(
			dateLocal.Year(), dateLocal.Month(), dateLocal.Day(),
			t.Hour(), t.Minute(), 0, 0,
			loc,
		)
	}

	dayStart := parseHM(wh.StartTime)
	dayEnd := parseHM(wh.EndTime)

	hasLunch := wh.LunchStart != "" && wh.LunchEnd != ""
	var lunchStart, lunchEnd time.Time
	if hasLunch {
		lunchStart = parseHM(wh.LunchStart)
		lunchEnd = parseHM(wh.LunchEnd)
	}

	// 4) Buscar appointments no range do dia
	appointments, err := uc.repo.ListAppointmentsForDay(
		ctx,
		in.BarbershopID,
		in.BarberID,
		dayStart,
		dayEnd,
	)
	if err != nil {
		return nil, err
	}

	// 5) Antecedência mínima — mesmo critério do checkout
	minAdvance := shop.MinAdvanceMinutes
	if minAdvance <= 0 {
		minAdvance = 120
	}
	earliest := time.Now().In(loc).Add(time.Duration(minAdvance) * time.Minute)

	// 6) Slots
	slotDuration := time.Duration(product.DurationMin) * time.Minute
	toleranceDur := time.Duration(shop.ScheduleToleranceMinutes) * time.Minute
	slots := make([]domain.TimeSlot, 0)

	apIdx := 0

	for cur := dayStart; cur.Add(slotDuration).Before(dayEnd) || cur.Add(slotDuration).Equal(dayEnd); cur = cur.Add(slotDuration) {
		slotStart := cur
		slotEnd := cur.Add(slotDuration)

		// Oculta slots que o checkout rejeitaria por antecedência insuficiente
		if slotStart.Before(earliest) {
			continue
		}

		if hasLunch && slotStart.Before(lunchEnd) && slotEnd.After(lunchStart) {
			continue
		}

		for apIdx < len(appointments) && !appointments[apIdx].EndTime.After(slotStart) {
			apIdx++
		}

		conflict := false
		if apIdx < len(appointments) {
			ap := appointments[apIdx]
			// Com tolerância: o slot só conflita se a sobreposição exceder o limite configurado.
			// slotStart + tol < ap.EndTime  →  início do slot (considerando margem) está antes do fim do existente
			// slotEnd - tol > ap.StartTime  →  fim do slot (considerando margem) está depois do início do existente
			effectiveStart := slotStart.Add(toleranceDur)
			effectiveEnd := slotEnd.Add(-toleranceDur)
			// Se a tolerância é maior que metade do serviço o range fica inválido;
			// recua para o range completo para não liberar slots ocupados.
			if !effectiveStart.Before(effectiveEnd) {
				effectiveStart = slotStart
				effectiveEnd = slotEnd
			}
			if effectiveStart.Before(ap.EndTime) && effectiveEnd.After(ap.StartTime) {
				conflict = true
			}
		}

		if !conflict {
			slots = append(slots, domain.TimeSlot{
				Start: slotStart.Format("15:04"),
				End:   slotEnd.Format("15:04"),
			})
		}
	}

	return slots, nil
}
