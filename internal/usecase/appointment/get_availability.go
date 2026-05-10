package appointment

import (
	"context"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/apperr"
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
		return nil, apperr.ErrBusiness("barbershop_not_found")
	}
	if productErr != nil || product == nil {
		return nil, apperr.ErrBusiness("product_not_found")
	}

	loc := timezone.Location(shop.Timezone)
	dateLocal := in.Date.In(loc)

	// 3) Expediente efetivo do dia: working hours + schedule override (se existir).
	// resolveWorkingHours aplica as mesmas regras usadas em CreatePrivateAppointment,
	// garantindo que disponibilidade e criação validem exatamente o mesmo expediente.
	ewh, err := resolveWorkingHours(ctx, uc.repo, in.BarbershopID, in.BarberID, dateLocal)
	if err != nil {
		return nil, err
	}
	if ewh == nil {
		return []domain.TimeSlot{}, nil
	}

	dayStart := parseHM(ewh.StartTime, dateLocal, loc)
	dayEnd := parseHM(ewh.EndTime, dateLocal, loc)

	hasLunch := ewh.LunchStart != "" && ewh.LunchEnd != ""
	var lunchStart, lunchEnd time.Time
	if hasLunch {
		lunchStart = parseHM(ewh.LunchStart, dateLocal, loc)
		lunchEnd = parseHM(ewh.LunchEnd, dateLocal, loc)
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

		// Verifica todos os appointments a partir de apIdx enquanto houver sobreposição
		// possível com o slot. Verificar apenas appointments[apIdx] era insuficiente:
		// quando a tolerância cria uma janela entre slotStart e effectiveStart,
		// o primeiro appointment poderia não conflitar mas um seguinte ainda pode.
		conflict := false
		effectiveStart, effectiveEnd := applyTolerance(slotStart, slotEnd, shop.ScheduleToleranceMinutes)
		for i := apIdx; i < len(appointments); i++ {
			ap := appointments[i]
			if !ap.StartTime.Before(effectiveEnd) {
				break // todos os appointments seguintes começam após o fim efetivo do slot
			}
			if effectiveStart.Before(ap.EndTime) && effectiveEnd.After(ap.StartTime) {
				conflict = true
				break
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
