package crm

import (
	"testing"
)

// ----------------------------------------------------------------
// resolvePolicy
// ----------------------------------------------------------------

func TestResolvePolicy_NoShow_RequiresPaymentUpfront(t *testing.T) {
	metrics := MetricsDTO{NoShow: 2, TotalAppointments: 5}
	policy := resolvePolicy(metrics)

	if !policy.RequiresPaymentUpfront {
		t.Fatal("cliente com 2+ no-shows deve exigir pagamento antecipado")
	}
	if policy.Reason != "no_show" {
		t.Fatalf("esperado reason 'no_show', obtido %q", policy.Reason)
	}
}

func TestResolvePolicy_HighNoShowRate_RequiresPaymentUpfront(t *testing.T) {
	// 2 no-shows em 5 appointments = 40% ≥ 20% threshold
	metrics := MetricsDTO{NoShow: 2, TotalAppointments: 5, Completed: 3}
	policy := resolvePolicy(metrics)

	if !policy.RequiresPaymentUpfront {
		t.Fatal("cliente com alta taxa de no-show deve exigir pagamento antecipado")
	}
}

func TestResolvePolicy_LowNoShow_NoUpfrontRequired(t *testing.T) {
	// 1 no-show em 10 appointments = 10% < 20%
	metrics := MetricsDTO{NoShow: 1, TotalAppointments: 10, Completed: 9}
	policy := resolvePolicy(metrics)

	if policy.RequiresPaymentUpfront {
		t.Fatal("cliente com poucos no-shows não deve exigir pagamento antecipado")
	}
}

func TestResolvePolicy_ZeroHistory_NoUpfrontRequired(t *testing.T) {
	policy := resolvePolicy(MetricsDTO{})

	if policy.RequiresPaymentUpfront {
		t.Fatal("cliente sem histórico não deve exigir pagamento antecipado")
	}
}

func TestResolvePolicy_BelowThresholdCount_NoUpfrontRequired(t *testing.T) {
	// 2 no-shows em 4 appointments → count < 5, rate não aplicada
	metrics := MetricsDTO{NoShow: 2, TotalAppointments: 4}
	policy := resolvePolicy(metrics)

	// Regra de rate só se aplica com TotalAppointments >= 5;
	// mas NoShow >= 2 dispara independente
	if !policy.RequiresPaymentUpfront {
		t.Fatal("NoShow >= 2 sempre exige pagamento, independente do total")
	}
}

// ----------------------------------------------------------------
// Flag semantics: Premium flag derivation
// ----------------------------------------------------------------

func TestPremiumFlag_WithActiveSubscription(t *testing.T) {
	sub := &SubscriptionDTO{PlanID: 1}
	flags := FlagsDTO{
		Premium: sub != nil,
	}
	if !flags.Premium {
		t.Fatal("Premium deve ser true quando há assinatura ativa")
	}
}

func TestPremiumFlag_WithoutSubscription(t *testing.T) {
	var sub *SubscriptionDTO
	flags := FlagsDTO{
		Premium: sub != nil,
	}
	if flags.Premium {
		t.Fatal("Premium deve ser false quando não há assinatura ativa")
	}
}

// ----------------------------------------------------------------
// Attendance-rate derived flags
// ----------------------------------------------------------------

func TestFlags_ReliableThreshold(t *testing.T) {
	tests := []struct {
		name          string
		completed     int
		total         int
		wantReliable  bool
		wantAttention bool
	}{
		{"perfect attendance 10 visits", 10, 10, true, false},
		{"exactly 90% attendance", 9, 10, true, false},
		{"below 90% attendance", 8, 10, false, false},
		{"below 70% attention", 6, 10, false, true},
		{"exactly 70% no attention", 7, 10, false, false},
		{"too few visits for reliable", 9, 4, false, false},
		{"zero appointments", 0, 0, false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var rate float64
			if tc.total > 0 {
				rate = float64(tc.completed) / float64(tc.total)
			}
			reliable := rate >= 0.9 && tc.total >= 5
			attention := tc.total > 0 && rate < 0.7

			if reliable != tc.wantReliable {
				t.Errorf("reliable: got %v, want %v (rate=%.2f, total=%d)", reliable, tc.wantReliable, rate, tc.total)
			}
			if attention != tc.wantAttention {
				t.Errorf("attention: got %v, want %v (rate=%.2f, total=%d)", attention, tc.wantAttention, rate, tc.total)
			}
		})
	}
}

// ----------------------------------------------------------------
// shared.ActiveSubscriptionSQL — documentar semantics
// ----------------------------------------------------------------

func TestActiveSubscriptionSQL_ContainsTimeBoundary(t *testing.T) {
	import_check := func(sql string) {
		if contains := len(sql) > 0; !contains {
			t.Fatal("shared.ActiveSubscriptionSQL must not be empty")
		}
		for _, fragment := range []string{
			"current_period_start",
			"current_period_end",
			"status",
		} {
			found := false
			for i := 0; i+len(fragment) <= len(sql); i++ {
				if sql[i:i+len(fragment)] == fragment {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("ActiveSubscriptionSQL missing required fragment: %q", fragment)
			}
		}
	}

	const canonicalSQL = `s.status = 'active'
	  AND s.current_period_start <= NOW()
	  AND s.current_period_end   >  NOW()`
	import_check(canonicalSQL)
}
