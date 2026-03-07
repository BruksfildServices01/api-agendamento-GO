package clienthistory

import (
	"context"

	domainMetrics "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
)

type Service struct {
	repo              *Repository
	getClientCategory *ucMetrics.GetClientCategory
}

func NewService(
	repo *Repository,
	getClientCategory *ucMetrics.GetClientCategory,
) *Service {
	return &Service{
		repo:              repo,
		getClientCategory: getClientCategory,
	}
}

func (s *Service) GetClientHistory(
	barbershopID int64,
	clientID int64,
) (*ClientHistoryDTO, error) {

	dto, err := s.repo.GetClientHistory(barbershopID, clientID)
	if err != nil {
		return nil, err
	}

	category, err := s.getClientCategory.Execute(
		context.Background(),
		uint(barbershopID),
		uint(clientID),
	)
	if err == nil {

		dto.Category = string(category)
	}

	if dto.AppointmentsTotal > 0 {
		dto.AttendanceRate =
			float64(dto.Attended) / float64(dto.AppointmentsTotal)
	}

	dto.Flags = buildFlags(dto)

	return dto, nil
}

func buildFlags(dto *ClientHistoryDTO) []ClientHistoryFlag {
	flags := make([]ClientHistoryFlag, 0)

	if dto.Category == string(domainMetrics.CategoryAtRisk) {
		flags = append(flags, FlagPaymentRequired)
	}

	if dto.AttendanceRate < 0.7 {
		flags = append(flags, FlagAttention)
	}

	if dto.AttendanceRate >= 0.9 && dto.AppointmentsTotal >= 5 {
		flags = append(flags, FlagReliable)
	}

	return flags
}
