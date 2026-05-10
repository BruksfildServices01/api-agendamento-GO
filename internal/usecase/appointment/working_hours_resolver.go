package appointment

import (
	"context"
	"time"

	domain "github.com/BruksfildServices01/barber-scheduler/internal/domain/appointment"
)

// effectiveWorkingHours representa o expediente efetivo do dia após aplicar
// working hours padrão e schedule override (quando existir).
type effectiveWorkingHours struct {
	StartTime  string // "HH:MM"
	EndTime    string // "HH:MM"
	LunchStart string // "" quando não há almoço
	LunchEnd   string // "" quando não há almoço
}

// resolveWorkingHours retorna o expediente efetivo do dia combinando working hours
// padrão com schedule override quando este existir.
//
// Retorna nil quando o dia está fechado por qualquer motivo:
//   - override.Closed = true
//   - working hours ausente ou inativo (sem override)
//   - working hours sem StartTime ou EndTime configurado
//
// Regras de resolução:
//  1. Busca working hours padrão para o weekday de dateLocal.
//  2. Busca schedule override: data específica tem prioridade sobre dia-da-semana/mês.
//  3. Se override.Closed → retorna nil (dia fechado).
//  4. Se override define StartTime/EndTime → usa horários do override, mas HERDA
//     LunchStart/LunchEnd do working hours original. Isso garante que o almoço
//     configurado não desapareça quando apenas o expediente é alterado via override.
//  5. Sem override → usa working hours original integralmente.
//
// Disponibilidade e criação de agendamento devem chamar esta função para garantir
// que validam exatamente o mesmo expediente efetivo.
func resolveWorkingHours(
	ctx context.Context,
	repo domain.Repository,
	barbershopID, barberID uint,
	dateLocal time.Time,
) (*effectiveWorkingHours, error) {

	weekday := int(dateLocal.Weekday())

	wh, err := repo.GetWorkingHours(ctx, barbershopID, barberID, weekday)
	if err != nil {
		return nil, err
	}

	override, err := repo.GetScheduleOverride(
		ctx,
		barbershopID,
		barberID,
		dateLocal.Format("2006-01-02"),
		weekday,
		int(dateLocal.Month()),
		dateLocal.Year(),
	)
	if err != nil {
		return nil, err
	}

	if override != nil {
		// Override fechado → dia sem expediente, independente do working hours.
		if override.Closed {
			return nil, nil
		}

		// Override com horários: substitui StartTime/EndTime mas herda o almoço
		// do working hours original para não expor horários de almoço como disponíveis.
		if override.StartTime != "" && override.EndTime != "" {
			lunchStart := ""
			lunchEnd := ""
			if wh != nil {
				lunchStart = wh.LunchStart
				lunchEnd = wh.LunchEnd
			}
			return &effectiveWorkingHours{
				StartTime:  override.StartTime,
				EndTime:    override.EndTime,
				LunchStart: lunchStart,
				LunchEnd:   lunchEnd,
			}, nil
		}
	}

	// Sem override aplicável: usa working hours padrão.
	if wh == nil || !wh.Active || wh.StartTime == "" || wh.EndTime == "" {
		return nil, nil // dia sem expediente configurado
	}

	return &effectiveWorkingHours{
		StartTime:  wh.StartTime,
		EndTime:    wh.EndTime,
		LunchStart: wh.LunchStart,
		LunchEnd:   wh.LunchEnd,
	}, nil
}
