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
	barbershopID uint,
	barberID uint,
	year int,
	month int,
) ([]dto.AppointmentListDTO, error) {

	// --------------------------------------------------
	// 1️⃣ Barbearia
	// --------------------------------------------------
	shop, err := uc.repo.GetBarbershopByID(ctx, barbershopID)
	if err != nil {
		return nil, err
	}
	if shop == nil {
		return nil, nil
	}

	loc := timezone.Location(shop.Timezone)

	// --------------------------------------------------
	// 2️⃣ Intervalo do mês
	// --------------------------------------------------
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)

	// --------------------------------------------------
	// 3️⃣ Buscar appointments
	// --------------------------------------------------
	appointments, err := uc.repo.ListAppointmentsForPeriod(
		ctx,
		barbershopID,
		barberID,
		start,
		end,
	)
	if err != nil {
		return nil, err
	}

	// --------------------------------------------------
	// 4️⃣ Mapear DTO (nil-safe)
	// --------------------------------------------------
	out := make([]dto.AppointmentListDTO, 0, len(appointments))

	for _, ap := range appointments {

		var clientName string
		if ap.Client != nil {
			clientName = ap.Client.Name
		}

		var productName string
		if ap.BarberProduct != nil {
			productName = ap.BarberProduct.Name
		}

		out = append(out, dto.AppointmentListDTO{
			ID:          ap.ID,
			StartTime:   ap.StartTime,
			EndTime:     ap.EndTime,
			Status:      string(ap.Status),
			ClientName:  clientName,
			ProductName: productName,
		})
	}

	return out, nil
}
