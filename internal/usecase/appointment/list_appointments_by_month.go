package appointment

import (
	"context"
	"time"

	"github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
	"github.com/BruksfildServices01/barber-scheduler/internal/dto"
	"github.com/BruksfildServices01/barber-scheduler/internal/timezone"
)

type ListAppointmentsByMonth struct {
	repo appointment.Repository
}

func NewListAppointmentsByMonth(
	repo appointment.Repository,
) *ListAppointmentsByMonth {
	return &ListAppointmentsByMonth{
		repo: repo,
	}
}

func (uc *ListAppointmentsByMonth) Execute(
	ctx context.Context,
	barberID uint,
	barbershopID uint,
	year int,
	month int,
) ([]dto.AppointmentListDTO, error) {

	shop, err := uc.repo.GetBarbershopByID(ctx, barbershopID)
	if err != nil {
		return nil, err
	}

	loc := timezone.Location(shop.Timezone)

	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)

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
