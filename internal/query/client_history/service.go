package clienthistory

import (
	"context"

	domainMetrics "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
	ucMetrics "github.com/BruksfildServices01/barber-scheduler/internal/usecase/metrics"
	ucSubscription "github.com/BruksfildServices01/barber-scheduler/internal/usecase/subscription"
)

type Service struct {
	repo                  *Repository
	getClientCategory     *ucMetrics.GetClientCategory
	getActiveSubscription *ucSubscription.GetActiveSubscription
}

func NewService(
	repo *Repository,
	getClientCategory *ucMetrics.GetClientCategory,
	getActiveSubscription *ucSubscription.GetActiveSubscription,
) *Service {
	return &Service{
		repo:                  repo,
		getClientCategory:     getClientCategory,
		getActiveSubscription: getActiveSubscription,
	}
}

func (s *Service) GetClientHistory(
	ctx context.Context,
	barbershopID int64,
	clientID int64,
) (*ClientHistoryDTO, error) {

	dto, err := s.repo.GetClientHistory(ctx, barbershopID, clientID)
	if err != nil {
		return nil, err
	}

	category, err := s.getClientCategory.Execute(
		ctx,
		uint(barbershopID),
		uint(clientID),
	)
	if err == nil {
		dto.Category = string(category)
	}

	if s.getActiveSubscription != nil {
		sub, err := s.getActiveSubscription.Execute(
			ctx,
			uint(barbershopID),
			uint(clientID),
		)
		if err == nil {
			dto.Premium = sub != nil
		}
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
