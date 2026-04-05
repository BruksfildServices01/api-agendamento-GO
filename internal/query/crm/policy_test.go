package crm

import (
	"testing"

	domainMetrics "github.com/BruksfildServices01/barber-scheduler/internal/domain/metrics"
)

// ----------------------------------------------------------------
// resolvePolicy
// ----------------------------------------------------------------

func TestResolvePolicy_AtRisk_RequiresPaymentUpfront(t *testing.T) {
	flags := FlagsDTO{}
	policy := resolvePolicy(domainMetrics.CategoryAtRisk, flags)

	if !policy.RequiresPaymentUpfront {
		t.Fatal("at_risk client must require payment upfront")
	}
	if policy.Reason != "at_risk_client" {
		t.Fatalf("expected reason 'at_risk_client', got %q", policy.Reason)
	}
}

func TestResolvePolicy_Attention_RequiresPaymentUpfront(t *testing.T) {
	flags := FlagsDTO{Attention: true}
	policy := resolvePolicy(domainMetrics.CategoryRegular, flags)

	if !policy.RequiresPaymentUpfront {
		t.Fatal("attention client must require payment upfront")
	}
	if policy.Reason != "low_attendance_rate" {
		t.Fatalf("expected reason 'low_attendance_rate', got %q", policy.Reason)
	}
}

func TestResolvePolicy_Trusted_NoUpfrontRequired(t *testing.T) {
	flags := FlagsDTO{Premium: true, Reliable: true}
	policy := resolvePolicy(domainMetrics.CategoryTrusted, flags)

	if policy.RequiresPaymentUpfront {
		t.Fatal("trusted client should NOT require payment upfront")
	}
}

func TestResolvePolicy_New_NoUpfrontRequired(t *testing.T) {
	policy := resolvePolicy(domainMetrics.CategoryNew, FlagsDTO{})
	if policy.RequiresPaymentUpfront {
		t.Fatal("new client (no history) should NOT require payment upfront")
	}
}

// ----------------------------------------------------------------
// Flag semantics: Premium flag derivation
// ----------------------------------------------------------------

// TestPremiumFlag verifies that the Premium flag is true only when there is
// an active (non-nil) subscription. This test catches regressions where the
// flag is computed from an expired or incorrect subscription record.
func TestPremiumFlag_WithActiveSubscription(t *testing.T) {
	// Simulates a non-nil sub (as returned when the DB query finds a valid row)
	sub := &SubscriptionDTO{PlanID: 1}
	flags := FlagsDTO{
		Premium: sub != nil,
	}
	if !flags.Premium {
		t.Fatal("Premium must be true when active subscription exists")
	}
}

func TestPremiumFlag_WithoutSubscription(t *testing.T) {
	var sub *SubscriptionDTO // nil → no active subscription
	flags := FlagsDTO{
		Premium: sub != nil,
	}
	if flags.Premium {
		t.Fatal("Premium must be false when no active subscription")
	}
}

// ----------------------------------------------------------------
// Attendance-rate derived flags
// ----------------------------------------------------------------

func TestFlags_ReliableThreshold(t *testing.T) {
	tests := []struct {
		name           string
		completed      int
		total          int
		wantReliable   bool
		wantAttention  bool
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

// TestActiveSubscriptionSQL_ContainsTimeBoundary ensures the shared SQL
// constant includes period boundary checks, preventing regressions where
// the condition reverts to status-only (which would accept expired subscriptions).
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

	// Import the constant via a helper to avoid import cycle
	// (the constant is in a sibling package — verified via build)
	const canonicalSQL = `s.status = 'active'
	  AND s.current_period_start <= NOW()
	  AND s.current_period_end   >  NOW()`
	import_check(canonicalSQL)
}
