package subscription

import (
	"context"
	"testing"
)

// ── Testes de validação de cuts_included ──────────────────────────────────────
//
// cuts_included = 0 era ambíguo (tratado como ilimitado em partes do sistema).
// A decisão de produto é: cuts_included deve ser sempre > 0.
// create_plan e update_plan devem rejeitar 0 e negativos com ErrInvalidCutsIncluded.
//
// Estratégia: passamos repo nil. A validação de cuts_included ocorre ANTES
// de qualquer chamada ao repositório. Para valores inválidos, o erro é retornado
// antes de qualquer acesso ao repo. Para valores válidos, a execução avança e
// pode entrar em panic ao tentar usar o repo nil — isso confirma que a validação
// de cuts_included passou (o erro não foi ErrInvalidCutsIncluded).

// runCreatePlanSafe executa uc.Execute e devolve o erro, capturando panic como
// indicação de que a validação passou e o repositório foi acessado.
func runCreatePlanSafe(uc *CreatePlan, ctx context.Context, input CreatePlanInput) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = nil // panic = passou da validação
		}
	}()
	_, err = uc.Execute(ctx, input)
	return
}

func runUpdatePlanSafe(uc *UpdatePlan, ctx context.Context, input UpdatePlanInput) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = nil // panic = passou da validação
		}
	}()
	err = uc.Execute(ctx, input)
	return
}

func TestCreatePlan_CutsIncluded_Validation(t *testing.T) {
	ctx := context.Background()
	uc := NewCreatePlan(nil)

	cases := []struct {
		name         string
		cutsIncluded int
		wantErr      error
	}{
		{"negativo rejeita", -1, ErrInvalidCutsIncluded},
		{"zero rejeita", 0, ErrInvalidCutsIncluded},
		{"um aceita", 1, nil},
		{"quatro aceita", 4, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := CreatePlanInput{
				BarbershopID:      1,
				Name:              "Plano Teste",
				MonthlyPriceCents: 5000,
				DurationDays:      30,
				CutsIncluded:      tc.cutsIncluded,
				DiscountPercent:   0,
				ServiceIDs:        []uint{1},
			}

			err := runCreatePlanSafe(uc, ctx, input)

			if tc.wantErr != nil {
				if err != tc.wantErr {
					t.Errorf("cuts_included=%d: esperado %v, obtido %v", tc.cutsIncluded, tc.wantErr, err)
				}
			} else {
				// nil err: passou da validação de cuts_included (repo panic foi capturado ou uc retornou outro erro)
				if err == ErrInvalidCutsIncluded {
					t.Errorf("cuts_included=%d: não deveria retornar ErrInvalidCutsIncluded", tc.cutsIncluded)
				}
			}
		})
	}
}

func TestUpdatePlan_CutsIncluded_Validation(t *testing.T) {
	ctx := context.Background()
	uc := NewUpdatePlan(nil)

	cases := []struct {
		name         string
		cutsIncluded int
		wantErr      error
	}{
		{"negativo rejeita", -1, ErrInvalidCutsIncluded},
		{"zero rejeita", 0, ErrInvalidCutsIncluded},
		{"um aceita", 1, nil},
		{"oito aceita", 8, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := UpdatePlanInput{
				BarbershopID:      1,
				PlanID:            1,
				Name:              "Plano Atualizado",
				MonthlyPriceCents: 5000,
				DurationDays:      30,
				CutsIncluded:      tc.cutsIncluded,
				DiscountPercent:   0,
				ServiceIDs:        []uint{1},
			}

			err := runUpdatePlanSafe(uc, ctx, input)

			if tc.wantErr != nil {
				if err != tc.wantErr {
					t.Errorf("cuts_included=%d: esperado %v, obtido %v", tc.cutsIncluded, tc.wantErr, err)
				}
			} else {
				if err == ErrInvalidCutsIncluded {
					t.Errorf("cuts_included=%d: não deveria retornar ErrInvalidCutsIncluded", tc.cutsIncluded)
				}
			}
		})
	}
}
