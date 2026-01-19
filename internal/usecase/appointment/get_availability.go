package appointment

import (
	"context"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/httperr"
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

	product, err := uc.repo.GetProduct(ctx, in.BarbershopID, in.ProductID)
	if err != nil {
		return nil, httperr.ErrBusiness("product_not_found")
	}

	weekday := int(in.Date.Weekday())

	wh, err := uc.repo.GetWorkingHours(ctx, in.BarberID, weekday)
	if err != nil || !wh.Active {
		return []domain.TimeSlot{}, nil
	}

	loc := in.Date.Location()

	parseHM := func(hm string) time.Time {
		t, _ := time.Parse("15:04", hm)
		return time.Date(
			in.Date.Year(), in.Date.Month(), in.Date.Day(),
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

	appointments, err := uc.repo.ListAppointmentsForDay(
		ctx,
		in.BarberID,
		dayStart,
		dayEnd,
	)
	if err != nil {
		return nil, err
	}

	slotDuration := time.Duration(product.DurationMin) * time.Minute
	var slots []domain.TimeSlot

	apIdx := 0

	for cur := dayStart; cur.Add(slotDuration).Before(dayEnd) || cur.Add(slotDuration).Equal(dayEnd); cur = cur.Add(slotDuration) {

		slotStart := cur
		slotEnd := cur.Add(slotDuration)

		// almoço
		if hasLunch && slotStart.Before(lunchEnd) && slotEnd.After(lunchStart) {
			continue
		}

		// avança agendamentos finalizados
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
