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
	barbershopID uint,
	barberID uint,
	date time.Time,
) ([]dto.AppointmentListDTO, error) {

	// --------------------------------------------------
	// 1️⃣ Carrega barbearia
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
	// 2️⃣ Intervalo do dia na timezone da barbearia
	// --------------------------------------------------
	start := time.Date(
		date.Year(),
		date.Month(),
		date.Day(),
		0, 0, 0, 0,
		loc,
	)
	end := start.Add(24 * time.Hour)

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
	// 4️⃣ Mapear DTO com segurança (nil-safe)
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
			Status:      string(ap.Status), // DTO normalmente é string
			ClientName:  clientName,
			ProductName: productName,
		})
	}

	return out, nil
}
