package appointment

import (
	"context"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/dto"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
)

type ListAppointmentsByDate struct {
	repo domain.Repository
}

func NewListAppointmentsByDate(
	repo domain.Repository,
) *ListAppointmentsByDate {
	return &ListAppointmentsByDate{
		repo: repo,
	}
}

func (uc *ListAppointmentsByDate) Execute(
	ctx context.Context,
	barberID uint,
	barbershopID uint,
	date time.Time,
) ([]dto.AppointmentListDTO, error) {

	shop, err := uc.repo.GetBarbershopByID(ctx, barbershopID)
	if err != nil {
		return nil, err
	}

	loc := timezone.Location(shop.Timezone)

	start := time.Date(
		date.Year(),
		date.Month(),
		date.Day(),
		0, 0, 0, 0,
		loc,
	)
	end := start.Add(24 * time.Hour)

	appointments, err := uc.repo.ListAppointmentsForPeriod(
		ctx,
		barberID,
		start,
		end,
	)
	if err != nil {
		return nil, err
	}

	out := make([]dto.AppointmentListDTO, 0, len(appointments))
	for _, ap := range appointments {
		out = append(out, dto.AppointmentListDTO{
			ID:          ap.ID,
			StartTime:   ap.StartTime,
			EndTime:     ap.EndTime,
			Status:      ap.Status,
			ClientName:  ap.Client.Name,
			ProductName: ap.BarberProduct.Name,
		})
	}

	return out, nil
}
