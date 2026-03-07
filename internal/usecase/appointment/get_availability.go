package appointment

import (
	"context"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
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

	// 1) Barbearia (timezone é fonte da verdade)
	shop, err := uc.repo.GetBarbershopByID(ctx, in.BarbershopID)
	if err != nil {
		return nil, err
	}
	if shop == nil {
		return nil, httperr.ErrBusiness("barbershop_not_found")
	}

	loc := timezone.Location(shop.Timezone)
	dateLocal := in.Date.In(loc)

	// 2) Produto
	product, err := uc.repo.GetProduct(ctx, in.BarbershopID, in.ProductID)
	if err != nil || product == nil {
		return nil, httperr.ErrBusiness("product_not_found")
	}

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

	// 5) Slots
	slotDuration := time.Duration(product.DurationMin) * time.Minute
	slots := make([]domain.TimeSlot, 0)

	apIdx := 0

	for cur := dayStart; cur.Add(slotDuration).Before(dayEnd) || cur.Add(slotDuration).Equal(dayEnd); cur = cur.Add(slotDuration) {
		slotStart := cur
		slotEnd := cur.Add(slotDuration)

		if hasLunch && slotStart.Before(lunchEnd) && slotEnd.After(lunchStart) {
			continue
		}

		for apIdx < len(appointments) && appointments[apIdx].EndTime.Before(slotStart) {
			apIdx++
		}

		conflict := false
		if apIdx < len(appointments) {
			ap := appointments[apIdx]
			if slotStart.Before(ap.EndTime) && slotEnd.After(ap.StartTime) {
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
